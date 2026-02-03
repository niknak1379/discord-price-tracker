package charts

import (
	database "priceTracker/Database"

	"github.com/jedib0t/go-pretty/v6/table"
)

func AggregateTable(itemArr []*database.Item) string {
	t := table.NewWriter()
	t.AppendHeader(table.Row{"7 Day Average"})
	t.AppendHeader(table.Row{
		"Name", "New Price", "Average Used Price",
		"Average Price Sold", "Lowest Used Price", "STDEV", "Unique Listings",
	})
	for _, Item := range itemArr {
		t.AppendRow(table.Row{
			Item.Name, Item.CurrentLowestPrice.Price,
			Item.SevenDayAggregate.AveragePrice, Item.SevenDayAggregate.AveragePriceWhenSold,
			Item.SevenDayAggregate.LowestPriceDuringTimePeriod, Item.SevenDayAggregate.PriceSTDEV,
			Item.SevenDayAggregate.UniqueListings,
		})
	}
	return t.Render()
}
