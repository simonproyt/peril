package main

import (
	"fmt"
	"os"
	"time"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerMove(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")
		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutComeSafe:
			return pubsub.Ack
		case gamelogic.MoveOutcomeMakeWar:
			warRecognition := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.GetPlayerSnap(),
			}
			warKey := fmt.Sprintf("%s.%s", routing.WarRecognitionsPrefix, gs.GetUsername())
			if err := pubsub.PublishJSON(ch, routing.ExchangePerilTopic, warKey, warRecognition); err != nil {
				fmt.Printf("Failed to publish war recognition: %v\n", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.MoveOutcomeSamePlayer:
			return pubsub.NackDiscard
		default:
			return pubsub.NackDiscard
		}
	}
}

func publishGameLog(ch *amqp.Channel, username, message string) error {
	logMsg := routing.GameLog{
		CurrentTime: time.Now(),
		Message:     message,
		Username:    username,
	}
	logKey := fmt.Sprintf("%s.%s", routing.GameLogSlug, username)
	return pubsub.PublishGob(ch, routing.ExchangePerilTopic, logKey, logMsg)
}

func handlerWar(gs *gamelogic.GameState, ch *amqp.Channel) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")
		outcome, winner, loser := gs.HandleWar(rw)
		var logMessage string
		switch outcome {
		case gamelogic.WarOutcomeOpponentWon, gamelogic.WarOutcomeYouWon:
			logMessage = fmt.Sprintf("%s won a war against %s", winner, loser)
		case gamelogic.WarOutcomeDraw:
			logMessage = fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
		}

		if logMessage != "" {
			if err := publishGameLog(ch, rw.Attacker.Username, logMessage); err != nil {
				fmt.Printf("Failed to publish game log: %v\n", err)
				return pubsub.NackRequeue
			}
		}

		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon, gamelogic.WarOutcomeYouWon, gamelogic.WarOutcomeDraw:
			return pubsub.Ack
		default:
			fmt.Printf("Unexpected war outcome: %v\n", outcome)
			return pubsub.NackDiscard
		}
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

	pubCh, err := conn.Channel()
	if err != nil {
		fmt.Printf("Failed to open RabbitMQ publish channel: %v\n", err)
		os.Exit(1)
	}
	defer pubCh.Close()

	if err := pubsub.SubscribeJSON(conn, routing.ExchangePerilDirect, pauseQueueName, routing.PauseKey, pubsub.TransientQueue, handlerPause(gs)); err != nil {
		fmt.Printf("Failed to subscribe to pause queue: %v\n", err)
		os.Exit(1)
	}

	if err := pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, moveQueueName, moveBindingKey, pubsub.TransientQueue, handlerMove(gs, pubCh)); err != nil {
		fmt.Printf("Failed to subscribe to move queue: %v\n", err)
		os.Exit(1)
	}

	warQueueName := "war"
	warBindingKey := fmt.Sprintf("%s.*", routing.WarRecognitionsPrefix)
	if err := pubsub.SubscribeJSON(conn, routing.ExchangePerilTopic, warQueueName, warBindingKey, pubsub.DurableQueue, handlerWar(gs, pubCh)); err != nil {
		fmt.Printf("Failed to subscribe to war queue: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Waiting for pause messages on queue %q, moves on queue %q, and war on queue %q...\n", pauseQueueName, moveQueueName, warQueueName)

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
