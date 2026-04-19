package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
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

	gamelogic.PrintServerHelp()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		switch words[0] {
		case "pause":
			fmt.Println("Sending pause message...")
			if err := pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true}); err != nil {
				fmt.Printf("Failed to publish pause message: %v\n", err)
			}
		case "resume":
			fmt.Println("Sending resume message...")
			if err := pubsub.PublishJSON(ch, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: false}); err != nil {
				fmt.Printf("Failed to publish resume message: %v\n", err)
			}
		case "quit":
			fmt.Println("Exiting server...")
			return
		default:
			fmt.Printf("I don't understand the command %q.\n", words[0])
		}

		select {
		case <-sigCh:
			fmt.Println("Signal received, shutting down server...")
			return
		default:
		}
	}
}
