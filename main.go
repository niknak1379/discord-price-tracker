package main

import (
	// "context"

	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	database "priceTracker/Database"
	scheduler "priceTracker/Scheduler"

	// crawler "priceTracker/Crawler"
	// database "priceTracker/Database"
	logger "priceTracker/Logger"

	// scheduler "priceTracker/Scheduler"

	// crawler "priceTracker/Crawler"
	discord "priceTracker/Discord"

	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(logger.Logger)
	var wg sync.WaitGroup
	godotenv.Load()
	// crawler.GetPrice("https://www.bhphotovideo.com/c/product/1752177-REG/fractal_design_fd_c_nor1c_02_north_mid_tower_atx_case.html", "span[class^='price_']")
	// crawler.GetPrice("https://www.newegg.com/fractal-design-atx-mid-tower-meshify-3-steel-pc-case-white-fd-c-mes3a-04/p/N82E16811352227", "li.price-current strong")
	discord.BotToken = os.Getenv("PUBLIC_KEY")
	ctx, cancel := context.WithCancel(context.Background())
	database.InitDB(ctx)
	// itemArr, err := crawler.EbayFailover("https://www.ebay.com/sch/i.html?_nkw=rtx%203060%20ti&LH_ItemCondition=3000|2020|2010|1500&_udhi=707&rt=nc&LH_BIN=1&_stpos=90274&_fcid=1", 1000, "rtx 3060 ti")
	// slog.Info("ebay test", slog.Any("itemArr", itemArr), slog.Any("err", err))
	go scheduler.SetChannelScheduler(ctx)
	wg.Go(func() {
		discord.Run(ctx)
	})

	// make the program run unless sigINT is recieved
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	slog.Info("Graceful Shutdown setup")

	<-stop
	slog.Info("Shutdown")
	cancel()
	wg.Wait()
}
