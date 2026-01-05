package main

import (
	// "context"

	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"

	database "priceTracker/Database"

	// crawler "priceTracker/Crawler"
	discord "priceTracker/Discord"

	"github.com/joho/godotenv"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var wg sync.WaitGroup
	godotenv.Load()
	// crawler.GetPrice("https://www.bhphotovideo.com/c/product/1752177-REG/fractal_design_fd_c_nor1c_02_north_mid_tower_atx_case.html", "span[class^='price_']")
	// crawler.GetPrice("https://www.newegg.com/fractal-design-atx-mid-tower-meshify-3-steel-pc-case-white-fd-c-mes3a-04/p/N82E16811352227", "li.price-current strong")
	discord.BotToken = os.Getenv("PUBLIC_KEY")
	ctx, cancel := context.WithCancel(context.Background())
	database.InitDB(ctx)
	// go scheduler.SetChannelScheduler(ctx)
	wg.Go(func() {
		discord.Run(ctx)
	}) 
	
	// make the program run unless sigINT is recieved
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("graceful shutdown setup")

	<-stop
	fmt.Println("recieved signal, shutting down")
	cancel()
	wg.Wait()
}
