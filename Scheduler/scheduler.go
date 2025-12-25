package scheduler

import (
	"context"
	"log"
	"math"
	"math/rand/v2"
	crawler "priceTracker/Crawler"
	database "priceTracker/Database"

	discord "priceTracker/Discord"
	"time"
)

func InitScheduler(ctx context.Context) {
	// -------------------- set timer for daily scrapping -------------//
	updateAllPrices()
	go func() {
		ticker := time.NewTicker(8 * time.Hour)
		log.Println("setting ticker in crawler")
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updateAllPrices()
				log.Println("ticking")
			}
		}
	}()
}
func updateAllPrices() {
	log.Println("updateAllPrices being fired")
	itemsArr := database.GetAllItems()
	for _, v := range itemsArr {
		date := time.Now()
		currLow := database.Price{
			Price: math.MaxInt,
			Url:   "Unavailable From All Sources",
		}
		var np = database.Price{}
		for _, t := range v.TrackingList {
			r := rand.IntN(10)
			timer := time.NewTimer(time.Duration(r) * time.Second)
			<-timer.C

			// updates the price from the price source in the pricearr list of
			// the document
			oldLow, err := database.GetLowestHistoricalPrice(v.Name)
			if err != nil {
				log.Print(err)
				continue
			}
			np, err = updatePrice(v.Name, t.URI, t.HtmlQuery, oldLow.Price, date)
			if currLow.Price > np.Price && err == nil {
				currLow = np
			}
		}
		// keeps track of current lowest price, if a new price has been found
		// and no errors encountered
		if currLow.Price != math.MaxInt32 {
			database.UpdateLowestPrice(v.Name, currLow)
		}
	}
}
func updatePrice(Name string, URI string, HtmlQuery string, oldLow int, date time.Time) (database.Price, error) {
	newPrice, err := crawler.GetPrice(URI, HtmlQuery)
	if err != nil || newPrice == 0 {
		log.Print("error getting price in updatePrice", err, newPrice)
		discord.CrawlErrorAlert(discord.Discord, Name, URI, err)
		return database.Price{}, err
	}
	p, _ := database.AddNewPrice(Name, URI, newPrice, oldLow, date)

	// notify discord if a new historical low has been achieved
	if oldLow > newPrice {
		discord.LowestPriceAlert(discord.Discord, Name, newPrice, oldLow, URI)
	}
	return p, err
}
