package main

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	connectionString := "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(connectionString)
	if err != nil {
		fmt.Printf("Failed to connect to RabbitMQ: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Println("Connected to RabbitMQ successfully")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	fmt.Println("Signal received, shutting down server...")
}
