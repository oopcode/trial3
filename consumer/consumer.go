package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
)

const (
	NumWorkers = 5
	RMQAddr    = "amqp://rabbit:5672/"
	DBAddr     = "pg:5432"
	RoutingKey = "trial3"
)

var (
	AccessTokenPattern = "^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-(8|9|a|b)[a-f0-9]{3}-[a-f0-9]{12}$"
	EventCodePattern   = "[A-Za-z0-9]{1,255}"
	EmailPattern       = `(^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$)`
	StreamTypes        = []string{"email", "sms", "push"}
	PhonePattern       = `\+7-[0-9]{3}-[0-9]{3}-[0-9]{2}-[0-9]{2}`
	PushPattern        = "[A-Za-z0-9]{12}"
)

func main() {
	var (
		err  error
		conn *amqp.Connection
	)
	err = retry(time.Minute, time.Second, func() error {
		conn, err = amqp.Dial(RMQAddr)
		return err
	})
	if err != nil {
		log.Panicf("failed to connect to RabbitMQ: %s", err)
	}
	defer conn.Close()

	log.Println("Connected to RabbitMQ")

	ch, err := conn.Channel()
	if err != nil {
		log.Panicf("Failed to open a channel: %s", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		RoutingKey,
		false,
		false,
		false,
		false,
		nil,
	)

	if err != nil {
		log.Panicf("Failed to declare queue %s: %s", RoutingKey, err)
	}

	msgs, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Panicf("Failed to register a consumer: %s", err)
	}

	var db *sql.DB
	err = retry(time.Minute, time.Second, func() error {
		var (
			DBUser     = "trial3"
			DBPassword = "trial3"
			DBName     = "trial3"
		)
		dbinfo := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
			DBUser, DBPassword, DBAddr, DBName)
		db, err = sql.Open("postgres", dbinfo)
		return err
	})
	if err != nil {
		log.Panicf("failed to connect to PostgreSQL: %s", err)
	}
	defer db.Close()

	wg := &sync.WaitGroup{}
	for i := 0; i < NumWorkers; i++ {
		wg.Add(1)
		go consume(msgs, db, wg)
	}

	wg.Wait()
}

func consume(msgs <-chan amqp.Delivery, db *sql.DB, wg *sync.WaitGroup) {
	defer wg.Done()

	for bMsg := range msgs {
		msg := &Message{}
		json.Unmarshal(bMsg.Body, msg)

		if err := msg.validate(); err != nil {
			log.Printf("Invalid message %s: %s", msg.AccessToken, err)
			continue
		}

		log.Printf("Received messsage %s", msg.AccessToken)

		toKey := fmt.Sprintf("person_%s", msg.StreamType)
		toVal, ok := msg.Data[toKey]
		if !ok {
			log.Printf("Failed to find %s in message %s", toKey, msg.AccessToken)
			continue
		}
		delete(msg.Data, toKey)

		data, err := json.Marshal(msg.Data)
		if err != nil {
			log.Printf("Failed to marshal data for message %s: %s", msg.AccessToken, err)
			continue
		}

		var insertID int
		err = db.QueryRow("INSERT INTO trial3(access_token, event_code, stream_type, sent_to, msg_data)"+
			" VALUES($1,$2,$3,$4,$5) returning id;",
			msg.AccessToken, msg.EventCode, msg.StreamType, toVal, string(data)).Scan(&insertID)
		if err != nil {
			log.Printf("Failed to store message %s: %s", msg.AccessToken, err)
		}
	}
}

type Message struct {
	AccessToken string            `json:"access_token"`
	EventCode   string            `json:"event_code"`
	StreamType  string            `json:"stream_type"`
	Data        map[string]string `json:"data"`
}

func (m *Message) validate() error {
	if ok, _ := regexp.MatchString(AccessTokenPattern, m.AccessToken); !ok {
		return fmt.Errorf("invalid access_token: %s", m.AccessToken)
	}

	if ok, _ := regexp.MatchString(EventCodePattern, m.EventCode); !ok {
		return fmt.Errorf("invalid event_code: %s", m.EventCode)
	}

	var streamTypeOk bool
	for _, streamType := range StreamTypes {
		if streamType == m.StreamType {
			streamTypeOk = true
			break
		}
	}
	if !streamTypeOk {
		return fmt.Errorf("invalid stream_type: %s", m.StreamType)
	}

	toKey := fmt.Sprintf("person_%s", m.StreamType)
	toVal, ok := m.Data[toKey]
	if !ok {
		return fmt.Errorf("failed to find %s based on stream_type %s", toKey, m.StreamType)

	}
	switch m.StreamType {
	case "email":
		if ok, _ := regexp.MatchString(EmailPattern, toVal); !ok {
			return fmt.Errorf("invalid email: %s", toVal)
		}
	case "sms":
		if ok, _ := regexp.MatchString(PhonePattern, toVal); !ok {
			return fmt.Errorf("invalid phone number: %s", toVal)
		}
	case "push":
		if ok, _ := regexp.MatchString(PushPattern, toVal); !ok {
			return fmt.Errorf("invalid push ID: %s", toVal)
		}
	}

	return nil
}

func retry(duration time.Duration, sleep time.Duration, cb func() error) error {
	var (
		t0 = time.Now()
		i  = 0
	)
	for {
		i++
		err := cb()
		if err == nil {
			return nil
		}

		delta := time.Now().Sub(t0)
		if delta > duration {
			return fmt.Errorf("after %d attempts (during %s), last error: %s", i, delta, err)
		}

		time.Sleep(sleep)

		log.Println("retrying after error:", err)
	}
}
