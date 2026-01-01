package scheduler

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	crawler "priceTracker/Crawler"
	database "priceTracker/Database"
	"time"

	discord "priceTracker/Discord"
)

func SetChannelScheduler(ctx context.Context) {
	// -------------------- set timer for daily scrapping -------------//
	println("printing tables:", database.Tables)
	for i := range database.Tables {
		updateAllPrices(i)
	}
	go func() {
		ticker := time.NewTicker(8 * time.Hour)
		log.Println("setting ticker in crawler")
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for i := range database.Tables {
					updateAllPrices(i)
				}
				log.Println("ticking")
			}
		}
	}()
}

func updateAllPrices(ChannelID string) {
	log.Println("updateAllPrices being fired for channel", ChannelID)
	itemsArr := database.GetAllItems(ChannelID)
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
			oldLow, err := database.GetLowestHistoricalPrice(v.Name, ChannelID)
			if err != nil {
				log.Print(err)
				continue
			}
			np, err = updatePrice(v.Name, t.URI, t.HtmlQuery, oldLow.Price, date, ChannelID)
			if currLow.Price > np.Price && err == nil {
				currLow = np
			}
		}
		// keeps track of current lowest price, if a new price has been found
		// and no errors encountered
		if currLow.Price != math.MaxInt32 {
			database.UpdateLowestPrice(v.Name, currLow, ChannelID)
		}
		handleEbayListingsUpdate(v.Name, currLow.Price, ChannelID)
	}
}

func updatePrice(Name string, URI string, HtmlQuery string, oldLow int, date time.Time, ChannelID string) (database.Price, error) {
	newPrice, err := crawler.GetPrice(URI, HtmlQuery)
	if err != nil || newPrice == 0 {
		log.Print("error getting price in updatePrice", err, newPrice)
		discord.CrawlErrorAlert(Name, URI, err, ChannelID)
		return database.Price{}, err
	}
	p, _ := database.AddNewPrice(Name, URI, newPrice, oldLow, date, ChannelID)

	// notify discord if a new historical low has been achieved
	if oldLow > newPrice {
		discord.LowestPriceAlert(Name, newPrice, oldLow, URI, ChannelID)
	}
	return p, err
}

func handleEbayListingsUpdate(Name string, Price int, ChannelID string) {
	ebayListings, err := crawler.GetSecondHandListings(Name, Price)
	if err != nil {
		discord.CrawlErrorAlert(Name, "ebay.com", err, ChannelID)
	}
	oldEbayListings, _ := database.GetEbayListings(Name, ChannelID)
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
				fmt.Println("calling new ebay listing with old price of ", oldPrice)
				discord.EbayListingPriceChangeAlert(newListing, oldPrice, ChannelID)
			} else {
				discord.NewEbayListingAlert(newListing, ChannelID)
			}
		}
	}
	err = database.UpdateEbayListings(Name, ebayListings, ChannelID)
	if err != nil {
		log.Print("error updaing DB in ebay listing", err, Name)
		discord.CrawlErrorAlert(Name, "www.ebay.com/DBError", err, ChannelID)
		return
	}
}
