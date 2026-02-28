package scheduler

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	database "priceTracker/Database"
)

type crawlDetails struct {
	Item   *database.Item
	Cancel context.CancelFunc
}

var (
	backUpAmazonQuery = "div#apex_desktop span.priceToPay"
	exludedFields     = []string{"PriceHistory", "ListingsHistory", "EbayListings"}
	activeRoutines    map[string]crawlDetails
)

// SetChannelScheduler initializes and runs the scheduler for all channels.
// It periodically checks for new/deleted items and updates tracked items.
//
// Parameters:
//   - ctx: the context for managing the scheduler lifecycle
func SetChannelScheduler(ctx context.Context) {
	slog.Info("first crawl start time", slog.Any("start time", time.Now()))

	activeRoutines = make(map[string]crawlDetails) // Track running goroutines
	// for running it once immediately on deployment
	//
	go func() {
		for _, Channel := range database.ChannelMap {
			itemsArr := database.GetAllItems(Channel.ChannelID, exludedFields)
			for _, item := range itemsArr {
				// this incident rate isnt really used for anything tho since its not passed down
				// after the update finishes
				I := initIncidentRate(item)
				updateSingleItem(item, Channel, I)
			}
		}
	}()
	// Initial load for scheduler this runs after the timers hit tho not immediately
	loadAndStartItems(ctx)

	// Check for new/deleted items every half hour
	refreshTicker := time.NewTicker(30 * time.Minute)
	defer refreshTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("channel scheduler stopping")
			// Cancel all item routines
			for _, crawl := range activeRoutines {
				crawl.Cancel()
			}
			return
		case <-refreshTicker.C:
			slog.Info("refreshing item list")
			loadAndStartItems(ctx)
		}
	}
}

func loadAndStartItems(ctx context.Context) {
	// for tracking items that have been deleted and are no longer
	// visible in the database.getallitmes call
	currentItems := make(map[string]bool)
	for _, Channel := range database.ChannelMap {
		itemsArr := database.GetAllItems(Channel.ChannelID, exludedFields)
		for _, item := range itemsArr {
			itemKey := item.ID.String()
			currentItems[itemKey] = true

			// Check if item already running and wether timer and suppression
			// status have changed
			if crawlDetails, ok := activeRoutines[itemKey]; ok {
				// Item exists, check if timer or suppression have changed
				slog.Info("cancel function found for item", slog.String("itemName", item.Name))
				// check weather tracking list was changed
				oldItem := crawlDetails.Item
				if HaveItemPropertiesChanged(item, oldItem) {
					slog.Info("item Properties changed, resetting goroutine")
					removeRoutine(crawlDetails.Item)
				} else {
					slog.Info("suppression and timer unchanged skipping")
					continue // Timer unchanged, skip
				}
			}

			// Start new routine for this item
			r := rand.IntN(240) + 60
			time.Sleep(time.Duration(r) * time.Second)
			addRoutine(ctx, item, Channel)
		}
		// delete if not found in current items
	}
	for itemKey, crawl := range activeRoutines {
		if _, ok := currentItems[itemKey]; !ok {
			slog.Info("stopping routine for deleted item", slog.String("item", itemKey))
			removeRoutine(crawl.Item)
		}
	}
}

func addRoutine(ctx context.Context, Item *database.Item, Channel *database.Channel) {
	itemKey := Item.ID.String()
	itemCtx, cancel := context.WithCancel(ctx)
	// Get new timer value
	newTimer := time.Duration(Item.Timer) * time.Hour
	if newTimer == 0 {
		newTimer = 8 * time.Hour
	}

	activeRoutines[itemKey] = crawlDetails{
		Cancel: cancel,
		Item:   Item,
	}
	slog.Info("Initializing Crawler Schedule",
		slog.String("item", Item.Name),
		slog.String("timer", newTimer.String()))
	go func(itemCtx context.Context, itemKey string) {
		itemCrawlRoutine(itemCtx, Item, Channel)
		// Clean up when routine exits
		delete(activeRoutines, itemKey)
	}(itemCtx, itemKey)
}

func removeRoutine(Item *database.Item) {
	itemKey := Item.ID.String()
	crawlDetails := activeRoutines[itemKey]
	crawlDetails.Cancel()
	delete(activeRoutines, itemKey)
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
		slog.String("interval", crawlInterval.String()),
	)

	ticker := time.NewTicker(crawlInterval)
	defer ticker.Stop()

	// initialize error notification rate limit
	I := initIncidentRate(item)

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping item crawl routine", slog.String("item", item.Name))
			return
		case <-ticker.C:
			go updateSingleItem(item, Channel, I)
		}
	}
}
