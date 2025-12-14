package main

import (
	"log"
	"os"
	"priceTracker/bot"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	bot.BotToken = os.Getenv("PUBLIC_KEY")
	t := database.trackingInfo{
		URI: "hi",
		htmlQuery: "hi",
	}
	arr := []*database.trackingInfo{&t}
	i := database.item{
		Name: "name",
		trackingList: arr,
	}
	database.addItem()
	bot.Run() // call the run function of bot/bot.go
}
