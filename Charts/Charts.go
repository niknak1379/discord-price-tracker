package charts

import (
	database "priceTracker/Database"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
)

func PriceHistoryChart(Name string, date time.Time){
	database.GetPriceHistory(Name, date)
	charts.NewLine()
}