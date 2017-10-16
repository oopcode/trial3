package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/streadway/amqp"
)

const (
	NumWorkers = 5
	//RMQAddr    = "amqp://rabbit:5672/"
	//DB_ADDR    = "pg:5432"
	DB_ADDR    = "localhost:5432"
	RMQAddr    = "amqp://localhost:5672/"
	RoutingKey = "trial3"
)

func main() {

	var (
		err  error
		conn *amqp.Connection
		db   *sql.DB
	)

	err = retry(time.Minute, time.Second, func() error {
		cbConn, cbErr := amqp.Dial(RMQAddr)
		conn = cbConn
		return cbErr
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

	err = retry(time.Minute, time.Second, func() error {
		var (
			DB_USER     = "trial3"
			DB_PASSWORD = "trial3"
			DB_NAME     = "trial3"
		)
		dbinfo := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
			DB_USER, DB_PASSWORD, DB_ADDR, DB_NAME)
		db, err = sql.Open("postgres", dbinfo)
		return err
	})
	if err != nil {
		log.Panicf("failed to connect to PostgreSQL: %s", err)
	}
	defer db.Close()

	for i := 0; i < NumWorkers; i++ {
		go consume(msgs, db)
	}

	select {}
}

func consume(msgs <-chan amqp.Delivery, db *sql.DB) {
	for bMsg := range msgs {
		inMsg := &InMessage{}
		json.Unmarshal(bMsg.Body, inMsg)

		log.Printf("> Received messsage %s", inMsg.AccessToken)

		toKey := fmt.Sprintf("person_%s", inMsg.StreamType)
		toVal, ok := inMsg.Data[toKey]
		if !ok {
			log.Printf("Failed to find %s in message %s", toKey, inMsg.AccessToken)
			continue
		}
		delete(inMsg.Data, toKey)

		data, err := json.Marshal(inMsg.Data)
		if err != nil {
			log.Printf("Failed to marshal data for message %s: %s", inMsg.AccessToken, err)
			continue
		}

		var insertID int
		err = db.QueryRow("INSERT INTO trial3(access_token, event_code, stream_type, sent_to, msg_data) VALUES($1,$2,$3,$4,$5) returning id;",
			inMsg.AccessToken, inMsg.EventCode, inMsg.StreamType, toVal, string(data)).Scan(&insertID)
		if err != nil {
			log.Printf("Failed to store message %s: %s", inMsg.AccessToken, err)
		}
	}
}

type InMessage struct {
	AccessToken string            `json:"access_token"`
	EventCode   string            `json:"event_code"`
	StreamType  string            `json:"stream_type"`
	Data        map[string]string `json:"data"`
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
