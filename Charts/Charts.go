package charts

import (
	"log/slog"
	"os"
	database "priceTracker/Database"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/snapshot-chromedp/render"
)

func PriceHistoryChart(Name string, date time.Time) error{
	line := charts.NewLine()
	time := time.Date(
		2009,                 // Year
		time.November,        // Month (use time.Month constants)
		17,                   // Day
		20,                   // Hour (24-hour format)
		34,                   // Minute
		58,                   // Second
		651387237,            // Nanosecond
		time.UTC,             // Location/Time Zone (use time.UTC or time.Local)
	)
	priceList, err := database.GetPriceHistory(Name, time)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	
	if err != nil{
		return line, err
	}
    line.SetGlobalOptions(
        charts.WithTitleOpts(opts.Title{
            Title:    "Price History by Store",
            Subtitle: Name,
        }),
        charts.WithTooltipOpts(opts.Tooltip{
            Show:    opts.Bool(true),
            Trigger: "axis",
        }),
		charts.WithAnimation(false),
        charts.WithLegendOpts(opts.Legend{
			Show:   opts.Bool(true),
			Bottom: "0%",  // ← Position at bottom
		}),
        charts.WithDataZoomOpts(opts.DataZoom{
            Type:  "inside",
            Start: 0,
            End:   100,
        }),
    )

    // Set series options to show data points
    line.SetSeriesOptions(
		charts.WithLineChartOpts(opts.LineChart{
			ShowSymbol: opts.Bool(true),
			SymbolSize: 10,  // ← Make points bigger
			Smooth:     opts.Bool(false),
		}),
	)

	var dates []string
    dateToIdx := make(map[string]int)
    urlToPrices := make(map[string]map[int]int) // url -> {dateIdx -> price}
    
    for _, price := range priceList {
        dateStr := price.Date.Format("Jan 02")
        
        // Add unique date
        if _, exists := dateToIdx[dateStr]; !exists {
            dateToIdx[dateStr] = len(dates)
            dates = append(dates, dateStr)
        }
        
        // Group by URL
        if urlToPrices[price.Url] == nil {
            urlToPrices[price.Url] = make(map[int]int)
        }
        urlToPrices[price.Url][dateToIdx[dateStr]] = price.Price
    }
	result := make(map[string][]opts.LineData)
    for url, pricesByIdx := range urlToPrices {
        aligned := make([]opts.LineData, len(dates))
        
        for i := range aligned {
            if price, exists := pricesByIdx[i]; exists {
                aligned[i] = opts.LineData{Value: price}
            } else {
                aligned[i] = opts.LineData{Value: nil}
            }
        }
        
        result[url] = aligned
    }
	logger.Info("dates", slog.Any("dates", dates))
	for url, data := range result {
		logger.Info("slog message", slog.Any("hi", data))
        line.AddSeries(url, data)
    }
	
	line.SetXAxis(dates)
	line.SetSeriesOptions(
    charts.WithLineChartOpts(opts.LineChart{
        ShowSymbol: opts.Bool(true),  // ← Show data point markers
    }),
	)
	render.MakeChartSnapshot(line.RenderContent(), "my-chart.png")
	
	
    return err
}
