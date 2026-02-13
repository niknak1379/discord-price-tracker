package charts

import (
	"errors"
	"log/slog"
	database "priceTracker/Database"
	"slices"
	"strings"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/snapshot-chromedp/render"
)

func getPriceHistory(Names []string, month int, ChannelID string) ([]*database.Price, error) {
	var priceList []*database.Price
	var err error
	for _, Name := range Names {
		priceArr, err := database.GetPriceHistory(Name, time.Now().AddDate(0, -month, 0), ChannelID)
		if err != nil {
			return priceList, err
		}
		for i := range priceArr {
			priceArr[i].Url = Name + " - " + ExtractDomainName(priceArr[i].Url)
		}
		priceList = append(priceList, priceArr...)

	}
	slices.SortFunc(priceList, func(a, b *database.Price) int {
		return a.Date.Compare(b.Date)
	})
	return priceList, err
}

// PriceHistoryChart generates a price history chart for one or more items.
// The chart is saved as "my-chart.png" in the working directory.
//
// Parameters:
//   - Names: list of item names to include in the chart
//   - month: how many months of history to display
//   - ChannelID: the Discord channel ID
//
// Returns any error encountered.
func PriceHistoryChart(Names []string, month int, ChannelID string) error {
	line := charts.NewLine()

	priceList, err := getPriceHistory(Names, month, ChannelID)
	if err != nil || len(priceList) == 0 {
		if len(priceList) == 0 {
			err = errors.New("no price history was found for the requested item")
		}
		return err
	}

	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: "Price Chart",
			TitleStyle: &opts.TextStyle{
				Color:      "Black",
				FontWeight: "bold",
				FontSize:   36,
				Padding:    10,
			},
			TextAlign: "center",
			Left:      "center",
			Top:       "20px",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "white",
			Width:           "1100px",
			Height:          "600px",
		}),
		charts.WithGridOpts(opts.Grid{
			Show:         opts.Bool(true),
			ContainLabel: opts.Bool(true),
			Top:          "100px",
			Bottom:       "100px",
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
	//slog.Info("dates", slog.Any("dates", dates))
	for url, data := range result {
		slog.Info("make chart, adding series data for", slog.Any(url, data))
		line.AddSeries(url, data)
	}

	line.SetXAxis(dates)
	line.SetSeriesOptions(
		charts.WithLineChartOpts(opts.LineChart{
			ShowSymbol: opts.Bool(true),
		}),
	)
	err = render.MakeChartSnapshot(line.RenderContent(), "my-chart.png")

	return err
}

// ExtractDomainName extracts the domain name from a URL.
//
// Parameters:
//   - url: the URL to extract the domain from
//
// Returns the domain name (e.g., "amazon" from "https://www.amazon.com/...").
func ExtractDomainName(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	// Remove www.
	url = strings.TrimPrefix(url, "www.")
	// Split by . and get first part
	parts := strings.Split(url, ".")
	return parts[0]
}

func IncidentsByDomainChart(data []database.DomainTimeSeries) error {
	if len(data) == 0 {
		return errors.New("no incident data available")
	}

	line := charts.NewLine()

	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title:    "Incidents by Domain Over Time",
			Subtitle: "Daily incident counts per domain",
			TitleStyle: &opts.TextStyle{
				Color:      "Black",
				FontWeight: "bold",
				FontSize:   28,
				Padding:    10,
			},
			TextAlign: "center",
			Left:      "center",
			Top:       "20px",
		}),
		charts.WithInitializationOpts(opts.Initialization{
			BackgroundColor: "white",
			Width:           "1100px",
			Height:          "600px",
		}),
		charts.WithGridOpts(opts.Grid{
			Show:         opts.Bool(true),
			ContainLabel: opts.Bool(true),
			Top:          "100px",
			Bottom:       "100px",
		}),
		charts.WithLegendOpts(
			opts.Legend{
				Bottom:  "0%",
				Show:    opts.Bool(true),
				Padding: 10,
			},
		),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
		}),
		charts.WithAnimation(false),
	)

	domainData := make(map[string]map[string]int)
	allDates := make(map[string]bool)

	for _, d := range data {
		dateStr := d.Date.Format("2006-01-02")
		allDates[dateStr] = true
		if domainData[d.Domain] == nil {
			domainData[d.Domain] = make(map[string]int)
		}
		domainData[d.Domain][dateStr] = d.Count
	}

	var dates []string
	for date := range allDates {
		dates = append(dates, date)
	}
	slices.Sort(dates)

	for domain, countByDate := range domainData {
		values := make([]opts.LineData, len(dates))
		for i, date := range dates {
			if count, ok := countByDate[date]; ok {
				values[i] = opts.LineData{Value: count}
			} else {
				values[i] = opts.LineData{Value: 0}
			}
		}
		line.AddSeries(domain, values)
	}

	line.SetXAxis(dates)
	line.SetSeriesOptions(
		charts.WithLineChartOpts(opts.LineChart{
			ShowSymbol:   opts.Bool(true),
			SymbolSize:   6,
			Smooth:       opts.Bool(false),
			ConnectNulls: opts.Bool(false),
		}),
	)

	err := render.MakeChartSnapshot(line.RenderContent(), "incidents_by_domain.png")
	return err
}
