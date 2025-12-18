package main

import (
	"context"
	"log"
	"os"
	crawler "priceTracker/Crawler"
	database "priceTracker/Database"
	"priceTracker/discord"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	discord.BotToken = os.Getenv("PUBLIC_KEY")
	ctx, cancel := context.WithCancel(context.Background())
	database.InitDB(ctx, cancel)
	go crawler.InitCrawler(ctx, cancel)
	discord.Run() // call the run function of bot/bot.go
	defer func() {
		if err = database.Client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
		cancel()
	}()
}
