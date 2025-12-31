package scheduler

import (
	"context"
	"log"
	"math"
	"math/rand/v2"
	crawler "priceTracker/Crawler"
	database "priceTracker/Database"
	"time"

	discord "priceTracker/Discord"
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
		np := database.Price{}
		for _, t := range v.TrackingList {
			r := rand.IntN(120)
			r += r + 60
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
		handleEbayListingsUpdate(v.Name, currLow.Price)
	}
}

func updatePrice(Name string, URI string, HtmlQuery string, oldLow int, date time.Time) (database.Price, error) {
	newPrice, err := crawler.GetPrice(URI, HtmlQuery)
	if err != nil || newPrice == 0 {
		log.Print("error getting price in updatePrice", err, newPrice)
		discord.CrawlErrorAlert(Name, URI, err)
		return database.Price{}, err
	}
	p, _ := database.AddNewPrice(Name, URI, newPrice, oldLow, date)

	// notify discord if a new historical low has been achieved
	if oldLow > newPrice {
		discord.LowestPriceAlert(Name, newPrice, oldLow, URI)
	}
	return p, err
}

func handleEbayListingsUpdate(Name string, Price int) {
	ebayListings, err := crawler.GetSecondHandListings(Name, Price)
	if err != nil {
		discord.CrawlErrorAlert(Name, "ebay.com", err)
	}
	oldEbayListings, _ := database.GetEbayListings(Name)
	ListingsMap := map[string]int{} // maps titles to price for checking if price exists or was updated
	for _, Listing := range oldEbayListings {
		ListingsMap[Listing.Title] = Listing.Price
	}
	for _, newListing := range ebayListings {
		oldPrice, ok := ListingsMap[newListing.Title]
		// if listing not found in the old list, or if price changed
		// ping discord
		if !ok || oldPrice != newListing.Price {
			if newListing.Price != oldPrice {
				discord.EbayListingPriceChangeAlert(newListing, oldPrice)
			} else {
				discord.NewEbayListingAlert(newListing)
			}
		}
	}
	err = database.UpdateEbayListings(Name, ebayListings)
	if err != nil {
		log.Print("error updaing DB in ebay listing", err, Name)
		discord.CrawlErrorAlert(Name, "www.ebay.com/DBError", err)
		return
	}
}
