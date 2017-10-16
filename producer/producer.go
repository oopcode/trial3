package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/lucasjones/reggen"
	"github.com/streadway/amqp"
)

const (
	RMQAddr    = "amqp://rabbit:5672/"
	RoutingKey = "trial3"
)

var (
	AccessTokenPattern = "^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-(8|9|a|b)[a-f0-9]{3}-[a-f0-9]{12}$"
	EventCodePattern   = "[A-Za-z0-9]{1,255}"
	EmailPattern       = `(^[a-zA-Z0-9_.+-]+@[a-zA-Z0-9-]+\.[a-zA-Z0-9-.]+$)`
	StreamTypes        = []string{"email", "sms", "push"}
	PhonePattern       = "+7-[0-9]{3}-[0-9]{3}-[0-9]{2}-[0-9]{2}"
	PushPattern        = "[A-Za-z0-9]{12}"
)

func main() {
	prod := NewProducer(RMQAddr, RoutingKey)
	if err := prod.Run(); err != nil {
		log.Fatalf("Failed to start producer: %s", err)
	}
}

type Producer struct {
	addr       string
	routingKey string
}

func NewProducer(addr, routingKey string) *Producer {
	return &Producer{addr: addr, routingKey: routingKey}
}

func (p *Producer) Run() error {
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

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to connect to open a channel: %s", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		p.routingKey,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to connect to declare a queue: %s", err)
	}

	for {
		var (
			msg     = NewMessage()
			body, _ = json.Marshal(msg)
		)

		err = ch.Publish(
			"",
			q.Name,
			false,
			false,
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        body,
			})
		if err != nil {
			log.Printf("Failed to publish message %+v: %s", msg, err)
		}

		log.Printf("Produced message %s", msg.AccessToken)
		time.Sleep(time.Second)
	}

	return nil
}

func (p *Producer) onError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

type Message struct {
	AccessToken string            `json:"access_token"`
	EventCode   string            `json:"event_code"`
	StreamType  string            `json:"stream_type"`
	Data        map[string]string `json:"data"`
}

func NewMessage() *Message {
	var (
		accessToken, _ = reggen.Generate(AccessTokenPattern, 20)
		eventCode, _   = reggen.Generate(EventCodePattern, 255)
		streamType     = StreamTypes[rand.Intn(len(StreamTypes))]
		personTo       string
	)

	switch streamType {
	case "email":
		personTo, _ = reggen.Generate(EmailPattern, 255)
	case "sms":
		personTo, _ = reggen.Generate(PhonePattern, 10)
	case "push":
		personTo, _ = reggen.Generate(PushPattern, 20)
	}

	return &Message{
		AccessToken: accessToken,
		EventCode:   eventCode,
		StreamType:  streamType,
		Data: map[string]string{
			"person_name":                        "John Doe",
			fmt.Sprintf("person_%s", streamType): personTo,
		},
	}
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
