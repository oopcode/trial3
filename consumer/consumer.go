package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"strings"

	"github.com/streadway/amqp"
)

const (
	NumWorkers = 5
	RMQAddr    = "amqp://rabbit:5672/"
	RoutingKey = "trial3"
)

func main() {
	conn, err := getConnection(time.Minute, time.Second)
	if err != nil {
		log.Panicf("failed to connect to RabbitMQ: %s", err)
	}
	defer conn.Close()

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

	for i := 0; i < NumWorkers; i++ {
		go consume(msgs)
	}

	select {}
}

func consume(msgs <-chan amqp.Delivery) {
	for bMsg := range msgs {
		msg := &InMessage{}
		json.Unmarshal(bMsg.Body, msg)

		log.Printf("> Received messsage %s", msg.AccessToken)

		outMsg := &OutMessage{
			AccessToken: msg.AccessToken,
			EventCode:   msg.EventCode,
			StreamType:  msg.StreamType,
		}

		for field, value := range msg.Data {
			if strings.Contains(field, msg.StreamType) {
				outMsg.To = value
				delete(msg.Data, field)
				break
			}
		}
	}
}

type InMessage struct {
	AccessToken string            `json:"access_token"`
	EventCode   string            `json:"event_code"`
	StreamType  string            `json:"stream_type"`
	Data        map[string]string `json:"data"`
}

type OutMessage struct {
	AccessToken string            `json:"access_token"`
	EventCode   string            `json:"event_code"`
	StreamType  string            `json:"stream_type"`
	To          string            `json:"to"`
	Data        map[string]string `json:"data"`
}

func getConnection(duration time.Duration, sleep time.Duration) (connection *amqp.Connection, err error) {
	var (
		t0 = time.Now()
		i  = 0
	)
	for {
		i++
		conn, err := amqp.Dial(RMQAddr)
		if err == nil {
			return conn, nil
		}

		delta := time.Now().Sub(t0)
		if delta > duration {
			return nil, fmt.Errorf("after %d attempts (during %s), last error: %s", i, delta, err)
		}

		time.Sleep(sleep)

		log.Println("retrying after error:", err)
	}
}
