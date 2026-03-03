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
			switch Event.Change {
			case Edit:
				itemKey := Event.Item.ID.String()
				if crawlDetails, ok := activeRoutines[itemKey]; ok {
					removeRoutine(Event.Item)
					addRoutine(ctx, Event.Item, crawlDetails.Channel)
				}
			case Remove:
				itemKey := Event.Item.ID.String()
				if _, ok := activeRoutines[itemKey]; ok {
					removeRoutine(Event.Item)
				}
			case Add:
				itemKey := Event.Item.ID.String()
				if crawlDetails, ok := activeRoutines[itemKey]; !ok {
					addRoutine(ctx, Event.Item, crawlDetails.Channel)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
