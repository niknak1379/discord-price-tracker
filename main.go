package main

import (
	"context"
	"log/slog"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sync"

	crawler "priceTracker/Crawler"
	database "priceTracker/Database"
	discord "priceTracker/Discord"
	scheduler "priceTracker/Scheduler"
	types "priceTracker/Types"

	logger "priceTracker/Logger"

	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(logger.Logger)
	// https://medium.com/@bobzsj87/demist-the-memory-ghost-d6b7cf45dd2a
	// not an actual memory leak its just docker being weird
	// go func() {
	// 	slog.Info("Starting pprof on :6060")
	// 	if err := http.ListenAndServe("0.0.0.0:6060", nil); err != nil {
	// 		slog.Error("pprof failed:", slog.Any("error", err))
	// 	}
	// }()
	godotenv.Load()
	// amazonTest()
	// BestBuyTest()
	// crawlerTest()
	// InchMeasurement()
	discord.BotToken = os.Getenv("PUBLIC_KEY")
	ctx, cancel := context.WithCancel(context.Background())
	database.InitDB(ctx)
	types.StartAttemptListener(database.SaveAttempt, ctx.Done())
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

func amazonTest() {
	i, err := crawler.GetPrice("https://www.amazon.com/dp/B0B3F8V4JG?ref=cm_sw_r_ud_dp_EX1QNBD4J564MEHGZ4Y1&ref_=cm_sw_r_ud_dp_EX1QNBD4J564MEHGZ4Y1&social_share=cm_sw_r_ud_dp_EX1QNBD4J564MEHGZ4Y1&language=en-US",
		"form#addToCart span.a-price-whole", true)
	slog.Info("price", slog.Int("int", i), slog.Any("error", err))
}

func BestBuyTest() {
	i, err := crawler.GetPrice("https://www.bestbuy.com/product/msi-mpg-322urx-qd-oled-32-quantum-dot-oled-uhd-240hz-0-03ms-gaming-monitor-with-hdr400-displayport-2-1a-hdmi-usb-black/J3P7TX99VT/sku/6614908?sb_share_source=PDP&ref=app_pdp&loc=pdp_page",
		"div[data-testid='price-block-customer-price']", true)
	slog.Info("price", slog.Int("int", i), slog.Any("error", err))
}

func crawlerTest() {
	//crawler.GetPrice("https://www.bhphotovideo.com/c/product/1752177-REG/fractal_design_fd_c_nor1c_02_north_mid_tower_atx_case.html",
	//"span[class^='price_']", true)
	//crawler.GetPrice("https://www.newegg.com/fractal-design-atx-mid-tower-meshify-3-steel-pc-case-white-fd-c-mes3a-04/p/N82E16811352227",
	//"li.price-current strong", true)
	// url := "https://www.ebay.com/sch/i.html?_nkw=Radeon+rx+9070+xt&LH_ItemCondition=3000%7C2030%7C2020%7C2010%7C2000%7C1500%7C1000_udlo%3D200&_udhi=801&_stpos=90274&_fcid=1&rt=nc&LH_All=1"
	// url := "https://www.ebay.com/sch/i.html?_nkw=sigma%20f%2F1.4%2030mm%20Sony&LH_ItemCondition=3000|2030|2020|2010|2000|1500|1000_udlo=104&rt=nc&_udhi=416&LH_ALL=1&_stpos=90274&_fcid=1"
	// itemArr, bids, err := crawler.EbayFailover(url,
	// 1000, "Radeon 9070 xt", []string{}, false)
	// itemArr, bids, err := crawler.GetEbayListings(
	// 	"sigma f/1.4 30mm sony",
	// 	1000,
	// 	[]string{},
	// 	false)
	// 	slog.Info("ebay test",
	// 		slog.Any("itemArr", itemArr),
	// 		slog.Any("bid arr", bids),
	// 		slog.Any("err", err))
}

func InchMeasurement() {
	slog.Info("=== Testing GetSecondHandListings with 32 inch monitor ===")
	listings, bids, err := crawler.GetSecondHandListings(
		[]string{"Samsung 32\" Odyssey OLED"},
		1000, 0, 0, 100,
		"Tech", "", false,
		[]string{},
	)

	slog.Info("Test results",
		slog.Int("listings_count", len(listings)),
		slog.Int("bids_count", len(bids)),
		slog.Any("error", err))
}
