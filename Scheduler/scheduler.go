package scheduler

import (
	"context"
	"fmt"
	"log/slog"
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
	now := time.Now()
	slog.Info("first crawl start time", slog.Any("start time", now))
	for _, Channel := range database.Coordinates {
		updateAllPrices(Channel)
	}
	finishTime := time.Since(now)
	s := fmt.Sprintf("first crawl took %.2f hours and %.2f minutes", finishTime.Hours(), finishTime.Minutes())
	slog.Debug(s)
	ticker := time.NewTicker(4 * time.Hour)
	slog.Info("setting ticker in crawler")
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, Channel := range database.Coordinates {
				updateAllPrices(Channel)
			}
			slog.Info("ticking")
		}
	}
}

func updateAllPrices(Channel database.Channel) {
	slog.Info("updateAllPrices being fired for channel", 
		slog.String("ChannelID", Channel.ChannelID))
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
				continue
			}
			np, err = updatePrice(v.Name, t.URI, t.HtmlQuery, oldLow, date, Channel.ChannelID, v.SuppressNotifications)
			if currLow.Price > np.Price && err == nil {
				currLow = np
			}
		}
		// keeps track of current lowest price, if a new price has been found
		// and no errors encountered
		if currLow.Price != math.MaxInt32 {
			database.UpdateLowestPrice(v.Name, currLow, Channel.ChannelID)
		}
		handleEbayListingsUpdate(v.Name, currLow.Price, v.Type, Channel, v.SuppressNotifications)
		database.UpdateAggregateReport(v.Name, Channel.ChannelID)
	}
}

func updatePrice(Name string, URI string, HtmlQuery string, oldLow database.Price, date time.Time, ChannelID string, Suppress bool) (database.Price, error) {
	newPrice, err := crawler.GetPrice(URI, HtmlQuery)
	if err != nil || newPrice == 0 {
		slog.Error("error getting price in updatePrice", slog.Any("Error", err), 
			slog.Int("Returned Price", newPrice))
		discord.CrawlErrorAlert(Name, URI, err, ChannelID)
		return database.Price{}, err
	}
	p, _ := database.AddNewPrice(Name, URI, newPrice, oldLow.Price, date, ChannelID)

	// notify discord if a new historical low has been achieved
	if oldLow.Price > newPrice && !Suppress {
		discord.LowestPriceAlert(Name, newPrice, oldLow, URI, ChannelID)
	}
	return p, err
}

func handleEbayListingsUpdate(Name string, Price int, Type string, Channel database.Channel, Suppress bool) {
	oldEbayListings, _ := database.GetEbayListings(Name, Channel.ChannelID)
	ListingsMap := map[string]*types.EbayListing{} // maps titles to price for checking if price exists or was updated
	for i := range oldEbayListings {
		ListingsMap[oldEbayListings[i].URL] = &oldEbayListings[i]
	}
	ebayListings, err := crawler.GetSecondHandListings(Name, Price,
		Channel.Lat, Channel.Long, Channel.Distance, Type, Channel.LocationCode)
	if err != nil {
		discord.CrawlErrorAlert(Name, "Second Hand Listings", err, Channel.ChannelID)
	}
	for i := range ebayListings {
		oldListing, ok := ListingsMap[ebayListings[i].URL]
		// if listing not found in the old list, or if price changed
		// ping discord
		//
		// update how long the listing has been online for
		if ok {
			ebayListings[i].Duration = oldListing.Duration + 4*time.Hour
		}
		if !Suppress && (!ok || oldListing.Price != ebayListings[i].Price) {
			if ok && ebayListings[i].Price != oldListing.Price {
				discord.EbayListingPriceChangeAlert(ebayListings[i], oldListing.Price, Channel.ChannelID)
			} else {
				discord.NewEbayListingAlert(ebayListings[i], Channel.ChannelID)
			}
		}
	}
	err = database.UpdateEbayListings(Name, ebayListings, Channel.ChannelID)
	if err != nil {
		slog.Error("error updaing DB in ebay listing", 
			slog.Any("Error", err), slog.String("Name", Name))
		discord.CrawlErrorAlert(Name, "www.ebay.com/DBError", err, Channel.ChannelID)
		return
	}
}
