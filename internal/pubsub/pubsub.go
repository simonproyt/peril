package pubsub

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int

const (
	DurableQueue SimpleQueueType = iota
	TransientQueue
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	body, err := json.Marshal(val)
	if err != nil {
		return err
	}

	return ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var buf bytes.Buffer
	encoder := gob.NewEncoder(&buf)
	if err := encoder.Encode(val); err != nil {
		return err
	}

	return ch.PublishWithContext(
		context.Background(),
		exchange,
		key,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/gob",
			Body:        buf.Bytes(),
		},
	)
}

type AckType int

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	durable := queueType == DurableQueue
	autoDelete := queueType == TransientQueue
	exclusive := queueType == TransientQueue

	args := amqp.Table{}
	if queueType == TransientQueue {
		args = amqp.Table{
			"x-dead-letter-exchange":    "peril_dlx",
			"x-dead-letter-routing-key": "peril_dlq",
		}
	}

	queue, err := ch.QueueDeclare(queueName, durable, autoDelete, exclusive, false, args)
	if err != nil {
		ch.Close()
		return nil, amqp.Queue{}, err
	}

	if err := ch.QueueBind(queue.Name, key, exchange, false, nil); err != nil {
		ch.Close()
		return nil, amqp.Queue{}, err
	}

	return ch, queue, nil
}

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
	unmarshaller func([]byte) (T, error),
) error {
	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}

	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		return err
	}

	deliveries, err := ch.Consume(
		queueName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		return err
	}

	go func() {
		for d := range deliveries {
			msg, err := unmarshaller(d.Body)
			if err != nil {
				fmt.Printf("Failed to unmarshal message: %v\n", err)
				if err := d.Nack(false, false); err != nil {
					fmt.Printf("Failed to nack-discard message: %v\n", err)
				}
				continue
			}

			switch handler(msg) {
			case Ack:
				if err := d.Ack(false); err != nil {
					fmt.Printf("Failed to ack message: %v\n", err)
				} else {
					fmt.Println("Message ACKed")
				}
			case NackRequeue:
				if err := d.Nack(false, true); err != nil {
					fmt.Printf("Failed to nack-requeue message: %v\n", err)
				} else {
					fmt.Println("Message NACKed and requeued")
				}
			case NackDiscard:
				if err := d.Nack(false, false); err != nil {
					fmt.Printf("Failed to nack-discard message: %v\n", err)
				} else {
					fmt.Println("Message NACKed and discarded")
				}
			default:
				if err := d.Nack(false, false); err != nil {
					fmt.Printf("Failed to nack-discard message: %v\n", err)
				} else {
					fmt.Println("Message NACKed and discarded")
				}
			}
		}
	}()

	return nil
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
	return subscribe(conn, exchange, queueName, key, queueType, handler, func(data []byte) (T, error) {
		var msg T
		return msg, json.Unmarshal(data, &msg)
	})
}

func SubscribeGob[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T) AckType) error {
	return subscribe(conn, exchange, queueName, key, queueType, handler, func(data []byte) (T, error) {
		var msg T
		err := gob.NewDecoder(bytes.NewReader(data)).Decode(&msg)
		return msg, err
	})
}
