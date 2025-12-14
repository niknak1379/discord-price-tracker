package main

import (
	"log"
	"os"
	"priceTracker/bot"
	"priceTracker/database"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	bot.BotToken = os.Getenv("PUBLIC_KEY")
	database.Run()
	bot.Run() // call the run function of bot/bot.go
}
