package charts

import (
	"strconv"
	"time"

	database "priceTracker/Database"

	"github.com/jedib0t/go-pretty/v6/table"
)

// ThisWeekAggregateTable generates a table with 7-day aggregate statistics for all items.
//
// Parameters:
//   - ChannelID: the Discord channel ID
//
// Returns a slice of formatted table strings for Discord display.
func ThisWeekAggregateTable(ChannelID string) []string {
	retArr := []string{}
	t := table.NewWriter()
	t.SetTitle("Seven Day Aggregate")
	t.AppendHeader(table.Row{
		"Name", "New", "AVG Used",
		"AVG Sold", "Lowest", "STDEV", "#",
	})
	itemArr := database.GetAllItems(ChannelID,
		[]string{"PriceHistory", "ListingsHistory", "EbayListings"})
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
				"Name", "New", "AVG Used",
				"AVG Sold", "Lowest", "STDEV", "#",
			})
		}
	}
	if t.Length() != 0 {
		retArr = append(retArr, "```"+t.Render()+"```")
	}
	return retArr
}

// CustomAggregateTable generates a table with aggregate statistics for a custom time period.
//
// Parameters:
//   - ChannelID: the Discord channel ID
//   - months: the number of months to aggregate
//
// Returns a slice of formatted table strings for Discord display.
func CustomAggregateTable(ChannelID string, months int) []string {
	retArr := []string{}
	t := table.NewWriter()
	t.SetTitle(strconv.Itoa(months) + " Month Aggregate")
	t.AppendHeader(table.Row{
		"Name", "New", "AVG Used",
		"AVG Sold", "Lowest", "STDEV", "#",
	})
	itemArr := database.GetAllItems(ChannelID,
		[]string{"PriceHistory", "ListingsHistory", "EbayListings"})
	for _, Item := range itemArr {
		aggregate, _ := database.GenerateSecondHandPriceReport(Item.Name, time.Now(), months*31, ChannelID)
		t.AppendRow(table.Row{
			Item.Name, Item.CurrentLowestPrice.Price,
			aggregate.AveragePrice, aggregate.AveragePriceWhenSold,
			aggregate.LowestPriceDuringTimePeriod, aggregate.PriceSTDEV,
			aggregate.UniqueListings,
		})
		if len(t.Render()) > 1500 {
			retArr = append(retArr, "```"+t.Render()+"```")
			t = table.NewWriter()
			t.SetTitle("Seven Day Aggregate")
			t.AppendHeader(table.Row{
				"Name", "New", "AVG Used",
				"AVG Sold", "Lowest", "STDEV", "#",
			})
		}
	}
	if t.Length() != 0 {
		retArr = append(retArr, "```"+t.Render()+"```")
	}
	return retArr
}
