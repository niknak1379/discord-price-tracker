package main

import (
	"context"
	"log"
	"os"
	database "priceTracker/Database"
	discord "priceTracker/Discord"
	scheduler "priceTracker/Scheduler"

	"github.com/joho/godotenv"
)

func main() {
	
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	godotenv.Load()
	
	discord.BotToken = os.Getenv("PUBLIC_KEY")
	ctx, cancel := context.WithCancel(context.Background())
	database.InitDB(ctx, cancel)
	
	go scheduler.InitScheduler(ctx, cancel)
	
	discord.Run() 
	defer func() {
		if err := database.Client.Disconnect(context.TODO()); err != nil {
			panic(err)
		}
		cancel()
	}()
}
