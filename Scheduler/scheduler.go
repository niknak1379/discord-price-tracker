package scheduler

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	database "priceTracker/Database"
)

type crawlDetails struct {
	Item    *database.Item
	Channel *database.Channel
	Cancel  context.CancelFunc
}

var (
	backUpAmazonQuery   = "div#apex_desktop span.priceToPay"
	exludedFields       = []string{"PriceHistory", "ListingsHistory", "EbayListings"}
	activeRoutines      map[string]crawlDetails
	activeRoutinesMutex sync.Mutex
)

// SetChannelScheduler initializes and runs the scheduler for all channels.
// It periodically checks for new/deleted items and updates tracked items.
//
// Parameters:
//   - ctx: the context for managing the scheduler lifecycle
func SetChannelScheduler(ctx context.Context) {
	slog.Info("first crawl start time", slog.Any("start time", time.Now()))

	activeRoutines = make(map[string]crawlDetails) // Track running goroutines
	itemChangeEventBusInit(ctx)
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
	refreshTicker := time.NewTicker(4 * time.Hour)
	defer refreshTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Info("channel scheduler stopping")
			// Cancel all item routines
			activeRoutinesMutex.Lock()
			for _, crawl := range activeRoutines {
				crawl.Cancel()
			}
			activeRoutinesMutex.Unlock()
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
			activeRoutinesMutex.Lock()
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
					activeRoutinesMutex.Unlock()
					continue // Timer unchanged, skip
				}
			}

			// Start new routine for this item
			r := rand.IntN(240) + 60
			time.Sleep(time.Duration(r) * time.Second)
			addRoutine(ctx, item, Channel)
			activeRoutinesMutex.Unlock()
		}
		// delete if not found in current items
	}
	activeRoutinesMutex.Lock()
	for itemKey, crawl := range activeRoutines {
		if _, ok := currentItems[itemKey]; !ok {
			slog.Info("stopping routine for deleted item", slog.String("item", itemKey))
			removeRoutine(crawl.Item)
		}
	}
	activeRoutinesMutex.Unlock()
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
		Cancel:  cancel,
		Item:    Item,
		Channel: Channel,
	}
	slog.Info("Initializing Crawler Schedule",
		slog.String("item", Item.Name),
		slog.String("timer", newTimer.String()))
	go itemCrawlRoutine(itemCtx, Item, Channel)
}

func removeRoutine(Item *database.Item) {
	slog.Info("removing crawl Routine for Item", slog.Any("item", Item))
	itemKey := Item.ID.String()
	if crawlDetails, ok := activeRoutines[itemKey]; ok && crawlDetails.Cancel != nil {
		crawlDetails.Cancel()
		delete(activeRoutines, itemKey)
	} else {
		slog.Warn("trying to remove non-existatnt crawl routine")
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
