package charts

import (
	"errors"
	"log/slog"
	"os"
	database "priceTracker/Database"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/snapshot-chromedp/render"
)

func PriceHistoryChart(Name string, month int) error {
	line := charts.NewLine()
	priceList, err := database.GetPriceHistory(Name, time.Now().AddDate(0, -month, 0))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err != nil || len(priceList) == 0 {
		if len(priceList) == 0 {
			err = errors.New("no price history was found for the requested item")
		}
		return err
	}

	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: Name,
			TitleStyle: &opts.TextStyle{
				Color:      "Black",
				FontWeight: "bold",
				FontSize:   36,
				Padding:    10,
			},
			TextAlign: "center",
			Left:      "center",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "white",
			Width:           "1000px",
			Height:          "800px",
		}),
		charts.WithLegendOpts(
			opts.Legend{
				Bottom:  "0%",
				Show:    opts.Bool(true),
				Padding: 10,
				TextStyle: &opts.TextStyle{
					Overflow: "truncate",
				},
			},
		),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithAnimation(false),
		charts.WithYAxisOpts(opts.YAxis{
			Scale: opts.Bool(true),
		}),
	)

	// Set series options to show data points
	line.SetSeriesOptions(
		charts.WithLineChartOpts(opts.LineChart{
			ShowSymbol:   opts.Bool(true),
			SymbolSize:   10,
			Smooth:       opts.Bool(false),
			ConnectNulls: opts.Bool(false),
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
			ShowSymbol: opts.Bool(true), // ← Show data point markers
		}),
	)
	err = render.MakeChartSnapshot(line.RenderContent(), "my-chart.png")

	return err
}
