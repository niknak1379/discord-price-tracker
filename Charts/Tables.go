package charts

import (
	database "priceTracker/Database"

	"github.com/jedib0t/go-pretty/v6/table"
)

func AggregateTable(itemArr []*database.Item) []string {
	retArr := []string{}
	t := table.NewWriter()
	t.SetTitle("Seven Day Aggregate")
	t.AppendHeader(table.Row{
		"Name", "New Price", "AVG Used",
		"AVG Sold", "Lowest", "STDEV", "Listings#",
	})
	for _, Item := range itemArr {
		t.AppendRow(table.Row{
			Item.Name, Item.CurrentLowestPrice.Price,
			Item.SevenDayAggregate.AveragePrice, Item.SevenDayAggregate.AveragePriceWhenSold,
			Item.SevenDayAggregate.LowestPriceDuringTimePeriod, Item.SevenDayAggregate.PriceSTDEV,
			Item.SevenDayAggregate.UniqueListings,
		})
		if len(t.Render()) > 1500 {
			retArr = append(retArr, "```"+t.Render()+"```")
			t = table.NewWriter()
			t.SetTitle("Seven Day Aggregate")
			t.AppendHeader(table.Row{
				"Name", "New Price", "AVG Used",
				"AVG Sold", "Lowest", "STDEV", "Listings#",
			})
		}
	}
	if t.Length() != 0 {
		retArr = append(retArr, "```"+t.Render()+"```")
	}
	return retArr
}
