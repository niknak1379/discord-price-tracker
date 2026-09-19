package scheduler

import (
	"context"

	database "priceTracker/Database"
)

const (
	Edit int = iota
	Remove
	Add
)

func itemChangeEventBusInit(ctx context.Context) {
	go ProcessItemChangeEvent(ctx)
}

func ProcessItemChangeEvent(ctx context.Context) {
	for {
		select {
		case Event := <-database.ItemChangeChannel:
			// Short critical sections only: Locked helpers, no sleeps or I/O.
			activeRoutinesMutex.Lock()
			switch Event.Change {
			case Edit:
				itemKey := Event.Item.ID.String()
				if crawlDetails, ok := activeRoutines[itemKey]; ok {
					removeRoutineLocked(Event.Item)
					addRoutineLocked(ctx, Event.Item, crawlDetails.Channel)
				}
			case Remove:
				itemKey := Event.Item.ID.String()
				if _, ok := activeRoutines[itemKey]; ok {
					removeRoutineLocked(Event.Item)
				}
			case Add:
				itemKey := Event.Item.ID.String()
				if _, ok := activeRoutines[itemKey]; !ok {
					addRoutineLocked(ctx, Event.Item, &Event.Channel)
				}
			}
			activeRoutinesMutex.Unlock()
		case <-ctx.Done():
			return
		}
	}
}
