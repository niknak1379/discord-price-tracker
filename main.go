package main

import (
	"context"
	"log"
	"os"
	database "priceTracker/Database"
	discord "priceTracker/Discord"
	scheduler "priceTracker/Scheduler"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	time.Local, _ = time.LoadLocation("America/Los_Angeles")
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	godotenv.Load()
	//crawler.GetPrice("https://www.bhphotovideo.com/c/product/1752177-REG/fractal_design_fd_c_nor1c_02_north_mid_tower_atx_case.html", "span[class^='price_']")
	//crawler.GetPrice("https://www.microcenter.com/product/665641/fractal-design-north-mesh-atx-mid-tower-computer-case-black-walnut", "#options-pricing2022")
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
