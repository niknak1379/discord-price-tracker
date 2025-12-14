package main

import (
	"context"
	"log"
	"os"
	database "priceTracker/Database"
	"priceTracker/bot"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	bot.BotToken = os.Getenv("PUBLIC_KEY")
	
	database.GetAllItems()
	bot.Run() // call the run function of bot/bot.go
	defer func() {
		if err = database.Client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
	}()
}
