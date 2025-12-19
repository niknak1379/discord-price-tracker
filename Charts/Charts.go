package charts

import (
	database "priceTracker/Database"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

func PriceHistoryChart(Name string, date time.Time) (*charts.Line, error){
	line := charts.NewLine()
	priceList, err := database.GetPriceHistory(Name, date)
	if err != nil{
		return line, err
	}
    line.SetGlobalOptions(
        charts.WithTitleOpts(opts.Title{
            Title: "Price History by Store",
			Subtitle: Name,
        }),
        charts.WithLegendOpts(opts.Legend{Show: true}),
        charts.WithTooltipOpts(opts.Tooltip{Show: true}),
    )
	var dates []string
    dateSet := make(map[string]bool)
	for _, prices := range priceList {
		for _, price := range prices.Prices {
			dateStr := price.Date.Format("Jan 02")
			if !dateSet[dateStr] {
				dateSet[dateStr] = true
				dates = append(dates, dateStr)
			}
		}
	}
	line.SetXAxis(dates)

    // Add series for each URL
    for _, prices := range priceList {
        items := make([]opts.LineData, len(prices.Prices))
        for i, price := range prices.Prices {
            items[i] = opts.LineData{Value: price.Price}
        }
        line.AddSeries(prices.Url, items)
    }

    return line, err
}