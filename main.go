package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"

	database "priceTracker/Database"
	discord "priceTracker/Discord"
	logger "priceTracker/Logger"
	scheduler "priceTracker/Scheduler"

	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(logger.Logger)
	godotenv.Load()
	// crawler.GetPrice("https://www.bhphotovideo.com/c/product/1752177-REG/fractal_design_fd_c_nor1c_02_north_mid_tower_atx_case.html", "span[class^='price_']")
	// crawler.GetPrice("https://www.newegg.com/fractal-design-atx-mid-tower-meshify-3-steel-pc-case-white-fd-c-mes3a-04/p/N82E16811352227", "li.price-current strong")
	// i, _ := crawler.GetPrice("https://www.amazon.com/dp/B0B3F8V4JG?ref=cm_sw_r_ud_dp_EX1QNBD4J564MEHGZ4Y1&ref_=cm_sw_r_ud_dp_EX1QNBD4J564MEHGZ4Y1&social_share=cm_sw_r_ud_dp_EX1QNBD4J564MEHGZ4Y1&language=en-US", "form#addToCart span.a-price-whole")
	// slog.Info("price", slog.Int("int", i))
	// itemArr, err := crawler.EbayFailover("https://www.ebay.com/sch/i.html?_nkw=rtx%203060%20ti&LH_ItemCondition=3000|2020|2010|1500&_udhi=707&rt=nc&LH_BIN=1&_stpos=90274&_fcid=1", 1000, "rtx 3060 ti")
	// slog.Info("ebay test", slog.Any("itemArr", itemArr), slog.Any("err", err))
	discord.BotToken = os.Getenv("PUBLIC_KEY")
	ctx, cancel := context.WithCancel(context.Background())
	database.InitDB(ctx)
	go scheduler.SetChannelScheduler(ctx)
	var wg sync.WaitGroup
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
