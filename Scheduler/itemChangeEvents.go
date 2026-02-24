package scheduler

import database "priceTracker/Database"

var itemChangeChannel chan database.Item

func itemChangeEventBusInit() {
	itemChangeChannel = make(chan database.Item, 100)
	go ProcessItemChangeEvent()
}

func ProcessItemChangeEvent() {
	for {
	}
}
