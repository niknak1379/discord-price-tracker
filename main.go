package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
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
	// crawler.GetPrice("https://www.bhphotovideo.com/c/product/1752177-REG/fractal_design_fd_c_nor1c_02_north_mid_tower_atx_case.html", "span[class^='price_']")
	//crawler.GetPrice("https://www.newegg.com/fractal-design-atx-mid-tower-meshify-3-steel-pc-case-white-fd-c-mes3a-04/p/N82E16811352227", "li.price-current strong")
	discord.BotToken = os.Getenv("PUBLIC_KEY")
	ctx, cancel := context.WithCancel(context.Background())
	database.InitDB(ctx, cancel)
	go scheduler.InitScheduler(ctx, cancel)

	discord.Run(ctx)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop
	fmt.Println("recieved signal, shutting down")

	defer func() {
		if err := database.Client.Disconnect(ctx); err != nil {
			panic(err)
		}
		cancel()
	}()
}
