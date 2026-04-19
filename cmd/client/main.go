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

func handlerMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) {
	return func(move gamelogic.ArmyMove) {
		defer fmt.Print("> ")
		gs.HandleMove(move)
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

	pauseQueueName := fmt.Sprintf("%s.%s", routing.PauseKey, username)
	moveQueueName := fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username)
	moveKey := fmt.Sprintf("%s.%s", routing.ArmyMovesPrefix, username)
	moveBindingKey := fmt.Sprintf("%s.*", routing.ArmyMovesPrefix)

	gs := gamelogic.NewGameState(username)

	if err := pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, pauseQueueName, routing.PauseKey, pubsub.TransientQueue, handlerPause(gs)); err != nil {
		fmt.Printf("Failed to subscribe to pause queue: %v\n", err)
		os.Exit(1)
	}

	if err := pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, moveQueueName, moveBindingKey, pubsub.TransientQueue, handlerMove(gs)); err != nil {
		fmt.Printf("Failed to subscribe to move queue: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Waiting for pause messages on queue %q and moves on queue %q...\n", pauseQueueName, moveQueueName)

	pubCh, err := conn.Channel()
	if err != nil {
		fmt.Printf("Failed to open RabbitMQ publish channel: %v\n", err)
		os.Exit(1)
	}
	defer pubCh.Close()

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
			move, err := gs.CommandMove(words)
			if err != nil {
				fmt.Printf("Failed to move unit: %v\n", err)
				continue
			}
			if err := pubsub.PublishJSON(pubCh, routing.ExchangePerilTopic, moveKey, move); err != nil {
				fmt.Printf("Failed to publish move: %v\n", err)
				continue
			}
			fmt.Println("Move published successfully")
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
