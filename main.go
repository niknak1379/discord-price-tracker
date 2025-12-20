package main

import (
	"context"
	"log"
	"os"
	database "priceTracker/Database"
	discord "priceTracker/Discord"

	"github.com/joho/godotenv"
)

func main() {
	
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	// comment for docker deployment
	godotenv.Load()
	
	discord.BotToken = os.Getenv("PUBLIC_KEY")
	ctx, cancel := context.WithCancel(context.Background())
	database.InitDB(ctx, cancel)
	// go crawler.InitCrawler(ctx, cancel)
	// charts.PriceHistoryChart("5070", 3)
	discord.Run() // call the run function of bot/bot.go
	defer func() {
		if err := database.Client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
		cancel()
	}()
}
