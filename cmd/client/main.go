package main

import (
	"fmt"
	"os"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) {
	return func(ps routing.PlayingState) {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
	}
}

func main() {
	connectionString := "amqp://guest:guest@localhost:5672/"

	conn, err := amqp.Dial(connectionString)
	if err != nil {
		fmt.Printf("Failed to connect to RabbitMQ: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		fmt.Printf("Failed to get username: %v\n", err)
		os.Exit(1)
	}

	queueName := fmt.Sprintf("%s.%s", routing.PauseKey, username)

	gs := gamelogic.NewGameState(username)

	if err := pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, queueName, routing.PauseKey, pubsub.TransientQueue, handlerPause(gs)); err != nil {
		fmt.Printf("Failed to subscribe to pause queue: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Waiting for pause messages on queue %q...\n", queueName)

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		switch words[0] {
		case "spawn":
			if err := gs.CommandSpawn(words); err != nil {
				fmt.Printf("Failed to spawn unit: %v\n", err)
			}
		case "move":
			if _, err := gs.CommandMove(words); err != nil {
				fmt.Printf("Failed to move unit: %v\n", err)
			} else {
				fmt.Println("Move succeeded")
			}
		case "status":
			gs.CommandStatus()
		case "help":
			gamelogic.PrintClientHelp()
		case "spam":
			fmt.Println("Spamming not allowed yet!")
		case "quit":
			gamelogic.PrintQuit()
			return
		default:
			fmt.Printf("I don't understand the command %q.\n", words[0])
		}
	}
}
