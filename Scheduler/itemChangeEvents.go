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
		// cases it has to handle
		// 1. item change
		// 2. new item and item removal
		case Event := <-database.ItemChangeChannel:
			switch Event.Change {
			// actually for edit i just have to update the object pointer i dont need to
			// fully restart,
			// NVM forgot map struct fields are not editable
			case Edit:
				itemKey := Event.Item.ID.String()
				Channel := activeRoutines[itemKey].Channel
				removeRoutine(Event.Item)
				addRoutine(ctx, Event.Item, Channel)
			case Remove:
				removeRoutine(Event.Item)
			case Add:
				itemKey := Event.Item.ID.String()
				addRoutine(ctx, Event.Item, activeRoutines[itemKey].Channel)
			}
		case <-ctx.Done():
			return
		}
	}
}
