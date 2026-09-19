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
		// Snapshot values, not the map header: iterating the live map
		// after Unlock races with setup/delete writers.
		database.ChannelLock.Lock()
		channels := make([]*database.Channel, 0, len(database.ChannelMap))
		for _, ch := range database.ChannelMap {
			channels = append(channels, ch)
		}
		database.ChannelLock.Unlock()
		for _, Channel := range channels {
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
	// Snapshot ChannelMap under short lock; DB I/O + sleeps below run
	// outside activeRoutinesMutex so Remove/Add events are never starved.
	database.ChannelLock.Lock()
	channels := make([]*database.Channel, 0, len(database.ChannelMap))
	for _, ch := range database.ChannelMap {
		channels = append(channels, ch)
	}
	database.ChannelLock.Unlock()
	for _, Channel := range channels {
		// DB call outside mutex.
		itemsArr := database.GetAllItems(Channel.ChannelID, exludedFields)
		for _, item := range itemsArr {
			itemKey := item.ID.String()

			// Fast decision under short lock.
			activeRoutinesMutex.Lock()
			existing, ok := activeRoutines[itemKey]
			needsReset := false
			if ok {
				if HaveItemPropertiesChanged(item, existing.Item) {
					slog.Info("item Properties changed, resetting goroutine",
						slog.String("itemName", item.Name))
					needsReset = true
				} else {
					slog.Info("suppression and timer unchanged skipping",
						slog.String("itemName", item.Name))
					currentItems[itemKey] = true
					activeRoutinesMutex.Unlock()
					continue
				}
			}
			activeRoutinesMutex.Unlock()

			if needsReset {
				// Cancel outside the decision lock; removeRoutine locks internally.
				removeRoutine(existing.Item)
			}

			// Stagger startups outside any lock.
			r := rand.IntN(240) + 60
			time.Sleep(time.Duration(r) * time.Second)

			// Re-check under lock before insert: event bus may have added
			// the same item while we slept.
			activeRoutinesMutex.Lock()
			if _, already := activeRoutines[itemKey]; already {
				activeRoutinesMutex.Unlock()
				currentItems[itemKey] = true
				continue
			}
			addRoutineLocked(ctx, item, Channel)
			activeRoutinesMutex.Unlock()
			currentItems[itemKey] = true
		}
	}
	// Collect deletions under short lock, remove outside it.
	activeRoutinesMutex.Lock()
	toDelete := make([]*database.Item, 0)
	for itemKey, crawl := range activeRoutines {
		if _, ok := currentItems[itemKey]; !ok {
			slog.Info("stopping routine for deleted item", slog.String("item", itemKey))
			toDelete = append(toDelete, crawl.Item)
		}
	}
	activeRoutinesMutex.Unlock()
	for _, it := range toDelete {
		removeRoutine(it)
	}
}

func addRoutineLocked(ctx context.Context, Item *database.Item, Channel *database.Channel) {
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

func addRoutine(ctx context.Context, Item *database.Item, Channel *database.Channel) {
	activeRoutinesMutex.Lock()
	addRoutineLocked(ctx, Item, Channel)
	activeRoutinesMutex.Unlock()
}

func removeRoutineLocked(Item *database.Item) {
	itemKey := Item.ID.String()
	if crawlDetails, ok := activeRoutines[itemKey]; ok && crawlDetails.Cancel != nil {
		crawlDetails.Cancel()
		delete(activeRoutines, itemKey)
	} else {
		slog.Warn("trying to remove non-existatnt crawl routine")
	}
}

func removeRoutine(Item *database.Item) {
	slog.Info("removing crawl Routine for Item", slog.Any("item", Item))
	activeRoutinesMutex.Lock()
	removeRoutineLocked(Item)
	activeRoutinesMutex.Unlock()
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
