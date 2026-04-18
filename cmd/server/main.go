package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
)

func main() {
	connectionString := "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(connectionString)
	if err != nil {
		fmt.Printf("Failed to connect to RabbitMQ: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		fmt.Printf("Failed to open RabbitMQ channel: %v\n", err)
		os.Exit(1)
	}
	defer ch.Close()

	fmt.Println("Connected to RabbitMQ successfully")

	if err := ch.ExchangeDeclare(
		routing.ExchangePerilDirect,
		"direct",
		false,
		false,
		false,
		false,
		nil,
	); err != nil {
		fmt.Printf("Failed to declare exchange: %v\n", err)
		os.Exit(1)
	}

	if err := pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true}); err != nil {
		fmt.Printf("Failed to publish pause message: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Published pause message to exchange")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println("Signal received, shutting down server...")
}
