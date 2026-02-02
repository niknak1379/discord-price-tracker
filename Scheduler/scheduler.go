package scheduler

import (
	"context"
	"log/slog"
	"math"
	"math/rand/v2"
	"time"

	crawler "priceTracker/Crawler"
	database "priceTracker/Database"
	types "priceTracker/Types"

	discord "priceTracker/Discord"
)

func SetChannelScheduler(ctx context.Context) {
	slog.Info("first crawl start time", slog.Any("start time", time.Now()))

	// Check for new/deleted items every hour
	refreshTicker := time.NewTicker(30 * time.Minute)
	defer refreshTicker.Stop()

	activeRoutines := make(map[string]context.CancelFunc) // Track running goroutines
	itemTimers := make(map[string]time.Duration)          // Track current timers
	itemSuppression := make(map[string]bool)              // trakc noti suppression
	itemTrackingList := make(map[string][]*database.TrackingInfo)
	itemLowestPrice := make(map[string]int)
	itemAdditionalNames := make(map[string][]string)
	for _, Channel := range database.ChannelMap {
		itemsArr := database.GetAllItems(Channel.ChannelID)
		for _, item := range itemsArr {
			updateSingleItem(item, Channel)
		}
	}
	// Initial load for scheduler this runs after the timers hit tho not immediately
	loadAndStartItems(ctx, activeRoutines, itemTimers, itemSuppression,
		itemTrackingList, itemLowestPrice, itemAdditionalNames)

	for {
		select {
		case <-ctx.Done():
			slog.Info("channel scheduler stopping")
			// Cancel all item routines
			for _, cancel := range activeRoutines {
				cancel()
			}
			return
		case <-refreshTicker.C:
			slog.Info("refreshing item list")
			loadAndStartItems(ctx, activeRoutines, itemTimers, itemSuppression,
				itemTrackingList, itemLowestPrice, itemAdditionalNames)
		}
	}
}

func loadAndStartItems(ctx context.Context,
	activeRoutines map[string]context.CancelFunc,
	itemTimers map[string]time.Duration,
	itemSuppression map[string]bool,
	itemTrackingList map[string][]*database.TrackingInfo,
	itemLowestPrice map[string]int,
	itemAdditionalNames map[string][]string,
) {
	for _, Channel := range database.ChannelMap {
		itemsArr := database.GetAllItems(Channel.ChannelID)
		routineExists := make(map[string]bool)
		for _, item := range itemsArr {
			itemKey := item.Name + "_" + Channel.ChannelID
			routineExists[itemKey] = true

			// Get new timer value
			newTimer := time.Duration(item.Timer) * time.Hour
			if newTimer == 0 {
				newTimer = 8 * time.Hour
			}

			// Check if item already running and wether timer and suppression
			// status have changed
			if cancel, ok := activeRoutines[itemKey]; ok {
				// Item exists, check if timer or suppression have changed
				slog.Info("cancel function found for item", slog.String("itemName", item.Name))
				oldSuppression, ok := itemSuppression[itemKey]
				oldTimer, ok2 := itemTimers[itemKey]
				oldTrackingList, ok3 := itemTrackingList[itemKey]
				oldLowestPrice, ok4 := itemLowestPrice[itemKey]
				oldAdditionalNames, ok5 := itemAdditionalNames[itemKey]
				// check weather tracking list was changed
				var wasTrackignListChanged bool
				if ok3 && len(oldTrackingList) == len(item.TrackingList) {
					for index := range oldTrackingList {
						if oldTrackingList[index].HtmlQuery != item.TrackingList[index].HtmlQuery ||
							oldTrackingList[index].URI != item.TrackingList[index].URI {
							wasTrackignListChanged = true
							break
						}
					}
				} else {
					wasTrackignListChanged = true
				}

				// resets go routine if timer value has changed
				// or if suppress notification
				// or if lowest price for second hand data collection
				// or tracking list has changed
				if (ok2 && oldTimer != newTimer) ||
					(ok && oldSuppression != item.SuppressNotifications) ||
					(ok4 && oldLowestPrice != item.CurrentLowestPrice.Price) ||
					(ok5 && len(oldAdditionalNames) != len(item.AlternateTrackingQueries)) ||
					wasTrackignListChanged {
					slog.Info("timer changed, suppression, trackingList, or price changed for item, restarting",
						slog.String("item", item.Name),
						slog.String("old_timer", oldTimer.String()),
						slog.String("new_timer", newTimer.String()),
						slog.Bool("oldSuppression", oldSuppression),
						slog.Bool("itemSuppression", item.SuppressNotifications),
						slog.Int("desiredPrice", item.CurrentLowestPrice.Price),
					)
					cancel()
					delete(activeRoutines, itemKey)
					delete(itemTimers, itemKey)
					delete(itemSuppression, itemKey)
					delete(itemTrackingList, itemKey)
					delete(itemLowestPrice, itemKey)
					delete(itemAdditionalNames, itemKey)
				} else {
					slog.Info("suppression and timer unchanged skipping")
					continue // Timer unchanged, skip
				}
			}

			// Start new routine for this item
			r := rand.IntN(240) + 60
			time.Sleep(time.Duration(r) * time.Second)

			// Create cancel context for this item
			itemCtx, cancel := context.WithCancel(ctx)
			activeRoutines[itemKey] = cancel
			itemTimers[itemKey] = newTimer
			itemSuppression[itemKey] = item.SuppressNotifications
			itemTrackingList[itemKey] = item.TrackingList
			itemLowestPrice[itemKey] = item.CurrentLowestPrice.Price
			itemAdditionalNames[itemKey] = item.AlternateTrackingQueries
			slog.Info("Initializing Crawler Schedule",
				slog.String("item", item.Name),
				slog.String("timer", newTimer.String()))
			go func(itemCtx context.Context, itemKey string) {
				itemCrawlRoutine(itemCtx, item, Channel)
				// Clean up when routine exits
				delete(activeRoutines, itemKey)
				delete(itemTimers, itemKey)
				delete(itemSuppression, itemKey)
				delete(itemTrackingList, itemKey)
				delete(itemLowestPrice, itemKey)
				delete(itemAdditionalNames, itemKey)
			}(itemCtx, itemKey)
		}
	}

	// Stop routines for deleted items
	currentItems := make(map[string]bool)
	for _, Channel := range database.ChannelMap {
		itemsArr := database.GetAllItems(Channel.ChannelID)
		for _, item := range itemsArr {
			itemKey := item.Name + "_" + Channel.ChannelID
			currentItems[itemKey] = true
		}
	}
	// delete if not found in current items
	for itemKey, cancel := range activeRoutines {
		if _, ok := currentItems[itemKey]; !ok {
			slog.Info("stopping routine for deleted item", slog.String("item", itemKey))
			cancel()
			delete(activeRoutines, itemKey)
			delete(itemTimers, itemKey)
		}
	}
}

func itemCrawlRoutine(ctx context.Context, item *database.Item, Channel *database.Channel) {
	// Random delay before first crawl
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

func updateSingleItem(item *database.Item, Channel *database.Channel) {
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

		// yesterdays lowest price
		oldLow := item.CurrentLowestPrice

		np, err := updatePrice(item.Name, t, oldLow, date, Channel.ChannelID, item.SuppressNotifications)
		if currLow.Price > np.Price && err == nil {
			currLow = np
		}
	}

	if currLow.Price == math.MaxInt {
		currLow.Price = item.CurrentLowestPrice.Price
	}

	item.CurrentLowestPrice = currLow
	database.UpdateLowestPrice(item.Name, &currLow, Channel.ChannelID)
	handleSecondHandListingsUpdate(item.Name, item.CurrentLowestPrice.Price,
		item.Type, Channel, item.SuppressNotifications, item.Timer, item.AlternateTrackingQueries)
	database.UpdateAggregateReport(item.Name, Channel.ChannelID)
}

func updatePrice(Name string, Tracker *database.TrackingInfo, oldLow database.Price, date time.Time, ChannelID string, Suppress bool) (database.Price, error) {
	newPrice, err := crawler.GetPrice(Tracker.URI, Tracker.HtmlQuery, true)
	if err != nil || newPrice == 0 {
		slog.Error("error getting price in updatePrice", slog.Any("Error", err),
			slog.Int("Returned Price", newPrice))
		discord.CrawlErrorAlert(Name, Tracker.URI, err, ChannelID)
		return database.Price{}, err
	}
	p, _ := database.AddNewPrice(Name, Tracker.URI, newPrice, date, ChannelID)

	// notify discord if price has changed more than %5
	if !Suppress && oldLow.Price != newPrice &&
		float64((oldLow.Price-newPrice)/oldLow.Price) > 0.05 {
		discord.PriceChangeAlert(Name, newPrice, oldLow, Tracker.URI, ChannelID)
	}
	return p, err
}

func handleSecondHandListingsUpdate(Name string, Price int, Type string,
	Channel *database.Channel, Suppress bool, timer int, additionalNames []string,
) {
	oldEbayListings, _ := database.GetEbayListings(Name, Channel.ChannelID)
	ListingsMap := map[string]*types.EbayListing{} // maps titles to price for checking if price exists or was updated
	for i := range oldEbayListings {
		ListingsMap[oldEbayListings[i].URL] = oldEbayListings[i]
	}
	ebayListings, err := crawler.GetSecondHandListings(Name, Price,
		Channel.Lat, Channel.Long, Channel.Distance,
		Type, Channel.LocationCode, additionalNames)
	if err != nil {
		discord.CrawlErrorAlert(Name, "Second Hand Listings", err, Channel.ChannelID)
	} else {
		for i := range ebayListings {
			oldListing, ok := ListingsMap[ebayListings[i].URL]
			// if listing not found in the old list, or if price changed
			// ping discord
			// update how long the listing has been online for
			if ok {
				if timer == 0 {
					timer = 8
				}
				ebayListings[i].Duration = oldListing.Duration + time.Duration(timer)*time.Hour
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
					if !Suppress &&
						math.Abs(float64(priceChange)) > 5 {
						discord.EbayListingPriceChangeAlert(ebayListings[i], oldListing.Price, Channel.ChannelID)
					}
				} else {
					// have to pass down the stats since im not doing a look up eachtime
					ebayListings[i].PriceDecreaseNum = oldListing.PriceDecreaseNum
					ebayListings[i].PriceIncreaseNum = oldListing.PriceIncreaseNum
					ebayListings[i].TotalPriceChange = oldListing.TotalPriceChange
				}
			} else if !Suppress {
				discord.NewEbayListingAlert(ebayListings[i], Channel.ChannelID)
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
}
