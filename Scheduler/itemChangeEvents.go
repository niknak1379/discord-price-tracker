package scheduler

import (
	"context"

	database "priceTracker/Database"
)

var itemChangeChannel chan itemChangeEvent

type (
	itemChangeEvent struct {
		Item   *database.Item
		Change int
	}
)

const (
	Edit int = iota
	Remove
	Add
)

func itemChangeEventBusInit() {
	itemChangeChannel = make(chan itemChangeEvent, 100)
	go ProcessItemChangeEvent(context.TODO())
}

func ProcessItemChangeEvent(ctx context.Context) {
	for {
		select {
		// cases it has to handle
		// 1. item change
		// 2. new item and item removal
		case Event := <-itemChangeChannel:
			switch Event.Change {
			case Edit:
				removeRoutine(ctx, Event.Item, ChannelID)
				addRoutine(ctx, Event.Item, ChannelID)
			case Remove:
				removeRoutine(ctx, Event.Item, ChannelID)
			case Add:
				addRoutine(ctx, Event.Item, ChannelID)
			}

		case <-ctx.Done():
			return
		}
	}
}
