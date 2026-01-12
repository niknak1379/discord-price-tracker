package scheduler

import (
	"context"
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
	slog.Info("first crawl start time", slog.Any("start time", time.Now()))

	for _, Channel := range database.Coordinates {
		itemsArr := database.GetAllItems(Channel.ChannelID)
		for _, item := range itemsArr {
			// Start goroutine for each item with staggered start
			r := rand.IntN(120) + 60
			time.Sleep(time.Duration(r) * time.Second)
			go itemCrawlRoutine(ctx, *item, Channel)
		}
	}

	<-ctx.Done()
	slog.Info("channel scheduler stopping")
}

func itemCrawlRoutine(ctx context.Context, item database.Item, Channel database.Channel) {
	// Random delay before first crawl (60-180 seconds)
	r := rand.IntN(120)
	time.Sleep(time.Duration(r) * time.Second)

	// Get item's timer or default to 8 hours
	crawlInterval := time.Duration(item.Timer) * time.Hour
	if crawlInterval == 0 {
		crawlInterval = 8 * time.Hour
	}

	slog.Info("starting item crawl routine",
		slog.String("item", item.Name),
		slog.String("interval", crawlInterval.String()))

	ticker := time.NewTicker(crawlInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping item crawl routine", slog.String("item", item.Name))
			return
		case <-ticker.C:
			updateSingleItem(item, Channel)
		}
	}
}

func updateSingleItem(item database.Item, Channel database.Channel) {
	slog.Info("updating item",
		slog.String("item", item.Name),
		slog.String("channelID", Channel.ChannelID))

	date := time.Now()
	currLow := database.Price{
		Price: math.MaxInt,
		Url:   "Unavailable From All Sources",
	}

	for _, t := range item.TrackingList {
		// Random delay between sources (60-180 seconds)
		r := rand.IntN(120) + 60
		time.Sleep(time.Duration(r) * time.Second)

		oldLow, err := database.GetLowestHistoricalPrice(item.Name, Channel.ChannelID)
		if err != nil {
			continue
		}

		np, err := updatePrice(item.Name, t.URI, t.HtmlQuery, oldLow, date, Channel.ChannelID, item.SuppressNotifications)
		if currLow.Price > np.Price && err == nil {
			currLow = np
		}
	}

	if currLow.Price != math.MaxInt32 {
		database.UpdateLowestPrice(item.Name, currLow, Channel.ChannelID)
	}

	handleEbayListingsUpdate(item.Name, currLow.Price, item.Type, Channel, item.SuppressNotifications)
	database.UpdateAggregateReport(item.Name, Channel.ChannelID)
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
