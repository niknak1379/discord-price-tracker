package scheduler

import (
	"log/slog"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	crawler "priceTracker/Crawler"
	database "priceTracker/Database"
	discord "priceTracker/Discord"
	proxy "priceTracker/Proxy"
	types "priceTracker/Types"
)

func updateSingleItem(item *database.Item, Channel *database.Channel, IR *IncidentRate) {
	slog.Info("updating item",
		slog.String("item", item.Name),
		slog.String("channelID", Channel.ChannelID))

	date := time.Now()
	// todays lowest price
	currLow := database.Price{
		Price: math.MaxInt,
		Url:   "Unavailable From All Sources",
		Date:  time.Now(),
	}

	for _, t := range item.TrackingList {
		// Random delay between sources (60-180 seconds)
		r := rand.IntN(180)
		time.Sleep(time.Duration(r) * time.Second)

		np, err := updatePrice(item.Name, t, date, Channel.ChannelID, IR)
		if currLow.Price > np.Price && err == nil {
			currLow = np
		}

	}

	if currLow.Price == math.MaxInt {
		currLow.Price = item.CurrentLowestPrice.Price
	}

	// notify discord if price has changed more than %5
	priceChangePercentage := math.Abs(float64(item.CurrentLowestPrice.Price-currLow.Price)) / float64(item.CurrentLowestPrice.Price)

	if item.CurrentLowestPrice.Price != currLow.Price {
		if !item.SuppressNotifications && priceChangePercentage > 0.05 {
			discord.PriceChangeAlert(item.Name, currLow.Price,
				item.CurrentLowestPrice, currLow.Url, Channel.ChannelID)
		}
		item.CurrentLowestPrice = currLow
		database.UpdateLowestPrice(item.Name, &currLow, Channel.ChannelID)
	}
	handleSecondHandListingsUpdate(item, Channel, IR)
	database.UpdateAggregateReport(item.Name, Channel.ChannelID)
}

func updatePrice(Name string, Tracker *database.TrackingInfo, date time.Time,
	ChannelID string, IR *IncidentRate,
) (database.Price, error) {
	// make copy of the proxy array
	proxyCopy := make([]string, len(proxy.ProxyList))
	copy(proxyCopy, proxy.ProxyList)
	newPrice, err := crawler.GetPrice(Name, Tracker.URI, Tracker.HtmlQuery, proxyCopy, nil)
	if err != nil && strings.Contains(Tracker.URI, "amazon") ||
		strings.Contains(Tracker.URI, "https://a.co") {
		proxyCopy := make([]string, len(proxy.ProxyList))
		copy(proxyCopy, proxy.ProxyList)
		newPrice, err = crawler.GetPrice(Name, Tracker.URI, backUpAmazonQuery, proxyCopy, nil)
	}
	if err != nil || newPrice == 0 {
		slog.Error("error getting price in updatePrice", slog.Any("Error", err),
			slog.Int("Returned Price", newPrice))
		IR.DefaultTrackers[Tracker.URI] += 1
		if HasIncidentRateLimitReached(IR) {
			discord.CrawlErrorAlert(Name, err, ChannelID)
		}
		return database.Price{}, err
	}
	IR.DefaultTrackers[Tracker.URI] = 0
	p, _ := database.AddNewPrice(Name, Tracker.URI, newPrice, date, ChannelID)

	return p, err
}

func handleSecondHandListingsUpdate(item *database.Item, Channel *database.Channel, incidentRate *IncidentRate) {
	oldEbayListings, err := database.GetEbayListings(item.Name, Channel.ChannelID)
	oldEbayBids, err1 := database.GetEbayBids(item.Name, Channel.ChannelID)
	if err != nil || err1 != nil {
		slog.Error("error getting from db",
			slog.Any("err", err),
			slog.Any("err1", err1),
		)
	}
	ListingsMap := map[string]*types.EbayListing{} // maps titles to price for checking if price exists or was updated
	BidsMap := map[string]*types.EbayBids{}
	for i := range oldEbayListings {
		ListingsMap[oldEbayListings[i].URL] = oldEbayListings[i]
	}
	for i := range oldEbayBids {
		BidsMap[oldEbayBids[i].URL] = oldEbayBids[i]
	}

	// set second hand price
	var SecondHandPrice int
	if item.SecondHandPrice == 0 {
		SecondHandPrice = item.CurrentLowestPrice.Price
	} else {
		SecondHandPrice = int(math.Min(float64(item.SecondHandPrice), float64(item.CurrentLowestPrice.Price)))
	}

	ebayListings, ebayBids, ebayErr, fbErr, depopErr := crawler.GetSecondHandListings(
		append(item.AlternateTrackingQueries, item.Name),
		SecondHandPrice,
		Channel.Lat, Channel.Long, Channel.Distance,
		item.Type, Channel.LocationCode, item.FacebookCrawl,
		item.TrackingExclusionQueries,
		proxy.ProxyList,
	)

	// Handle eBay errors with rate limiting
	if ebayErr != nil {
		slog.Warn("Second-hand eBay crawl failed", slog.String("item", item.Name))
		incidentRate.EbayTracker++
		if HasIncidentRateLimitReached(incidentRate) {
			discord.CrawlErrorAlert(item.Name, ebayErr, Channel.ChannelID)
		}
	} else {
		incidentRate.EbayTracker = 0
	}

	// Handle Facebook errors with rate limiting
	if fbErr != nil {
		slog.Warn("Second-hand Facebook crawl failed", slog.String("item", item.Name))
		incidentRate.FacebookTracker++
		if HasIncidentRateLimitReached(incidentRate) {
			discord.CrawlErrorAlert(item.Name, fbErr, Channel.ChannelID)
		}
	} else {
		incidentRate.FacebookTracker = 0
	}

	// Handle Depop errors with rate limiting
	if depopErr != nil {
		slog.Warn("Second-hand Depop crawl failed", slog.String("item", item.Name))
		incidentRate.DepopTracker++
		if HasIncidentRateLimitReached(incidentRate) {
			discord.CrawlErrorAlert(item.Name, depopErr, Channel.ChannelID)
		}
	} else {
		incidentRate.DepopTracker = 0
	}

	// Only process listings if we got some data (even if there were some errors)
	if len(ebayListings) > 0 || len(ebayBids) > 0 {
		if !item.SuppressNotifications {
			for i := range ebayBids {
				oldBid, ok := BidsMap[ebayBids[i].URL]
				if !ok {
					discord.NewBidAlert(ebayBids[i], Channel.ChannelID, &item.SevenDayAggregate)
				} else {
					if oldBid.Price != ebayBids[i].Price {
						discord.BidPriceChangeAlert(ebayBids[i], oldBid, Channel.ChannelID, &item.SevenDayAggregate)
					}
				}
			}
		}
		for i := range ebayListings {
			oldListing, ok := ListingsMap[ebayListings[i].URL]
			// if listing not found in the old list, or if price changed
			// ping discord
			// update how long the listing has been online for
			if ok {
				if item.Timer == 0 {
					item.Timer = 8
				}
				ebayListings[i].Duration = oldListing.Duration + time.Duration(item.Timer)*time.Hour
				if ebayListings[i].Price != oldListing.Price {
					// update count for how many times price was increased
					priceChange := ebayListings[i].Price - oldListing.Price
					ebayListings[i].TotalPriceChange = oldListing.TotalPriceChange + priceChange

					// price inc
					if priceChange > 0 {
						ebayListings[i].PriceIncreaseNum = oldListing.PriceIncreaseNum + 1
						ebayListings[i].PriceDecreaseNum = oldListing.PriceDecreaseNum
					} else {
						// price decrease
						ebayListings[i].PriceDecreaseNum = oldListing.PriceDecreaseNum + 1
						ebayListings[i].PriceIncreaseNum = oldListing.PriceIncreaseNum
					}
					if !item.SuppressNotifications &&
						math.Abs(float64(priceChange)) > 5 {
						discord.EbayListingPriceChangeAlert(ebayListings[i], oldListing.Price,
							Channel.ChannelID, &item.SevenDayAggregate)
					}
				} else {
					// have to pass down the stats since im not doing a look up eachtime
					ebayListings[i].PriceDecreaseNum = oldListing.PriceDecreaseNum
					ebayListings[i].PriceIncreaseNum = oldListing.PriceIncreaseNum
					ebayListings[i].TotalPriceChange = oldListing.TotalPriceChange
				}
			} else if !item.SuppressNotifications {
				discord.NewEbayListingAlert(ebayListings[i], Channel.ChannelID, &item.SevenDayAggregate)
			}
		}
		item.EbayBids = ebayBids
		item.EbayListings = ebayListings
		err = database.UpdateEbayListings(item.Name, ebayListings, Channel.ChannelID)
		err2 := database.UpdateEbayBids(item.Name, ebayBids, Channel.ChannelID)
		if err != nil || err2 != nil {
			slog.Error("error updaing DB in ebay listing",
				slog.Any("Error", err), slog.String("Name", item.Name))
		}
	}
}
