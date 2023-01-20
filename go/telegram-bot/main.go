package main

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var token = "token"

func main() {

	load()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	opts := []bot.Option{
		bot.WithDefaultHandler(handler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		panic(err)
	}

	b.Start(ctx)
}

var complains = []string{}

func load() {

	readFile, err := os.Open("msg.txt")

	if err != nil {
		fmt.Println(err)
	}
	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		complains = append(complains, fileScanner.Text())
	}

	fmt.Println("Loaded", len(complains), "messages")

	readFile.Close()

}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {

	n := rand.Intn(len(complains))

	msg := complains[n]

	fmt.Println("ID", update.Message.Chat.ID)
	fmt.Println("OD", update.Message)

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   msg,
	})
}
