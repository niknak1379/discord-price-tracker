package scheduler

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	crawler "priceTracker/Crawler"
	database "priceTracker/Database"
	types "priceTracker/Types"
	"time"

	discord "priceTracker/Discord"
)

func SetChannelScheduler(ctx context.Context) {
	// -------------------- set timer for daily scrapping -------------//
	println("printing tables:", database.Tables)
	now := time.Now()
	fmt.Println("first crawl start time", now)
	for _, Channel := range database.Coordinates {
		updateAllPrices(Channel)
	}
	finishTime := time.Since(now)
	log.Printf("first crawl took %.2f hours and %.2f minutes", finishTime.Hours(), finishTime.Minutes())

	ticker := time.NewTicker(4 * time.Hour)
	log.Println("setting ticker in crawler")
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, Channel := range database.Coordinates {
				updateAllPrices(Channel)
			}
			log.Println("ticking")
		}
	}
}

func updateAllPrices(Channel database.Channel) {
	log.Println("updateAllPrices being fired for channel", Channel.ChannelID)
	itemsArr := database.GetAllItems(Channel.ChannelID)
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
			time.Sleep(time.Duration(r) * time.Second)

			// updates the price from the price source in the pricearr list of
			// the document
			oldLow, err := database.GetLowestHistoricalPrice(v.Name, Channel.ChannelID)
			if err != nil {
				log.Print(err)
				continue
			}
			np, err = updatePrice(v.Name, t.URI, t.HtmlQuery, oldLow, date, Channel.ChannelID)
			if currLow.Price > np.Price && err == nil {
				currLow = np
			}
		}
		// keeps track of current lowest price, if a new price has been found
		// and no errors encountered
		if currLow.Price != math.MaxInt32 {
			database.UpdateLowestPrice(v.Name, currLow, Channel.ChannelID)
		}
		handleEbayListingsUpdate(v.Name, currLow.Price, v.Type, Channel)
		database.UpdateAggregateReport(v.Name, Channel.ChannelID)
	}
}

func updatePrice(Name string, URI string, HtmlQuery string, oldLow database.Price, date time.Time, ChannelID string) (database.Price, error) {
	newPrice, err := crawler.GetPrice(URI, HtmlQuery)
	if err != nil || newPrice == 0 {
		log.Print("error getting price in updatePrice", err, newPrice)
		discord.CrawlErrorAlert(Name, URI, err, ChannelID)
		return database.Price{}, err
	}
	p, _ := database.AddNewPrice(Name, URI, newPrice, oldLow.Price, date, ChannelID)

	// notify discord if a new historical low has been achieved
	if oldLow.Price > newPrice {
		discord.LowestPriceAlert(Name, newPrice, oldLow, URI, ChannelID)
	}
	return p, err
}

func handleEbayListingsUpdate(Name string, Price int, Type string, Channel database.Channel) {
	oldEbayListings, _ := database.GetEbayListings(Name, Channel.ChannelID)
	ListingsMap := map[string]types.EbayListing{} // maps titles to price for checking if price exists or was updated
	for _, Listing := range oldEbayListings {
		ListingsMap[Listing.Title] = Listing
	}
	ebayListings, err := crawler.GetSecondHandListings(Name, Price,
		Channel.Lat, Channel.Long, Channel.Distance, Type)
	if err != nil {
		discord.CrawlErrorAlert(Name, "ebay.com", err, Channel.ChannelID)
	}
	for _, newListing := range ebayListings {
		oldListing, ok := ListingsMap[newListing.Title]
		// if listing not found in the old list, or if price changed
		// ping discord
		if !ok || oldListing.Price != newListing.Price {
			if ok && newListing.Price != oldListing.Price {
				fmt.Println("calling new ebay listing with old price of and new price of", oldListing.Price, newListing.Price)
				discord.EbayListingPriceChangeAlert(newListing, oldListing.Price, Channel.ChannelID)
			} else {
				discord.NewEbayListingAlert(newListing, Channel.ChannelID)
			}
		}
	}
	err = database.UpdateEbayListings(Name, ebayListings, Channel.ChannelID)
	if err != nil {
		log.Print("error updaing DB in ebay listing", err, Name)
		discord.CrawlErrorAlert(Name, "www.ebay.com/DBError", err, Channel.ChannelID)
		return
	}
}
