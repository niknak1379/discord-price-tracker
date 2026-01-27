package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	types "priceTracker/Types"

	"github.com/chromedp/chromedp"
)

func GetSecondHandListings(Name string, Price int, homeLat float64, homeLong float64,
	maxDistance int, itemType string, LocationCode string,
) ([]types.EbayListing, error) {
	var depop []types.EbayListing
	var err3 error
	if itemType == "Clothes" {
		Price = Price / 2
		depop, err3 = CrawlDepop(Name, Price)
	}
	fb, err2 := MarketPlaceCrawl(Name, Price, homeLat, homeLong, maxDistance, LocationCode, true)
	ebay, err := GetEbayListings(Name, Price, true)

	if err != nil || err2 != nil || err3 != nil {
		slog.Error("errors from getting second hand listing",
			slog.Any("Ebay error", err),
			slog.Any("Facebook Marketplace Error", err2),
			slog.Any("Depop Error", err3),
		)
	}
	retArr := append(ebay, fb...)
	retArr = append(retArr, depop...)
	return retArr, errors.Join(err, err2, err3)
}

func FacebookURLGenerator(Name string, Price int, LocationCode string) string {
	baseURL := "https://www.facebook.com/marketplace/107711145919004/search"
	priceQuery := fmt.Sprintf("?maxPrice=%d", Price)
	query := "&query=" + url.PathEscape(Name) + "&exact=false"
	return baseURL + priceQuery + query
}

// JS loaded cannot use colly for this
func MarketPlaceCrawl(Name string, desiredPrice int, homeLat, homeLong float64,
	maxDistance int, LocationCode string, proxy bool,
) ([]types.EbayListing, error) {
	crawlDate := time.Now()
	url := FacebookURLGenerator(Name, desiredPrice, LocationCode)
	slog.Info("crawling facebook marketplace URL", slog.String("URL", url))
	var ctx context.Context
	var cancel context.CancelFunc
	if proxy {
		ctx, cancel = NewChromedpContext(90*time.Second, chromedp.ProxyServer("http://gluetun:8888"))
	} else {
		ctx, cancel = NewChromedpContext(90 * time.Second)
	}
	defer cancel()

	ctx, timeoutCancel := context.WithTimeout(ctx, 90*time.Second) // Increased timeout
	defer timeoutCancel()
	var first []byte
	var second []byte
	var HTMLContent string
	var items []types.EbayListing
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		StealthActions(),
		chromedp.Sleep(time.Duration(rand.IntN(10)+15)*time.Second),
		chromedp.FullScreenshot(&first, 70),
		chromedp.OuterHTML("body", &HTMLContent),
		chromedp.Evaluate(`document.querySelector('div.xdg88n9.x10l6tqk.x1tk7jg1.x1vjfegm')?.click()`, nil),
		chromedp.Sleep(3*time.Second),
		chromedp.FullScreenshot(&second, 70),
		chromedp.Evaluate(`
		Array.from(document.querySelectorAll('div.x9f619.x78zum5.x1r8uery.xdt5ytf.x1iyjqo2.xs83m0k.x135b78x.x11lfxj5.x1iorvi4.xjkvuk6.xnpuxes.x1cjf5ee.x17dddeq')).map(e => ({
				Title: e.querySelector('span.x1lliihq.x6ikm8r.x10wlt62.x1n2onr6')?.innerText || '',
				URL: e.querySelector('a')?.href || '',
				Price: ((el) => {
						if (!el || !el.innerText) return 0;
						const text = el.innerText.replaceAll('$', '').replaceAll(',', '');
						return parseInt(text) || 0;
				})(e.querySelector('span.x193iq5w.xeuugli.x13faqbe.x1vvkbs.xlh3980.xvmahel.x1n0sxbx.x1lliihq.x1s928wv.xhkezso.x1gmr53x.x1cpjm7i.x1fgarty.x1943h6x.x4zkp8e.x3x7a5m.x1lkfr7t.x1lbecb7.x1s688f.xzsf02u')),
				Condition: e.querySelector('span.x1lliihq.x6ikm8r.x10wlt62.x1n2onr6.xlyipyv.xuxw1ft')?.innerText || '',
		}))
		`, &items),
	)

	var retArr []types.EbayListing
	if err != nil || len(items) == 0 {
		if proxy {
			fileErr1 := os.WriteFile("proxyFacebookFirst.png", first, 0o644)
			fileErr2 := os.WriteFile("proxyFacebookSecond.png", second, 0o644)
			fileErr3 := os.WriteFile("proxyFacebookHTML.html", []byte(HTMLContent), 0o644)
			slog.Warn("facebook proxy failed, triggering no proxy crawl",
				slog.Any("Error", err),
				slog.Int("ItemArr length", len(items)),
				slog.Any("wirte Err", fileErr1),
				slog.Any("write err 2", fileErr2),
				slog.Any("write err 3", fileErr3),
			)
			retArr, err = MarketPlaceCrawl(Name, desiredPrice, homeLat, homeLong, maxDistance, LocationCode, false)
			return retArr, err
		} else {
			fileErr1 := os.WriteFile("facebookFirst.png", first, 0o644)
			fileErr2 := os.WriteFile("facebookSecond.png", second, 0o644)
			fileErr3 := os.WriteFile("facebookHTML.html", []byte(HTMLContent), 0o644)
			slog.Error("Error in marketplace", slog.Any("error value", err),
				slog.Any("File error 1", fileErr1),
				slog.Any("file error 2", fileErr2),
				slog.Any("file error 3", fileErr3),
			)
			err = errors.Join(errors.New("Error in facebook marketplace:"), err)
			return retArr, err
		}
	}
	// <------------------ sanitize the list ------------>
	for i := range items {
		if titleCorrectnessCheck(items[i].Title, Name) && items[i].Price != 0 &&
			items[i].Price < desiredPrice {
			distance, distStr, err := ValidateDistance(items[i].Condition, homeLat,
				homeLong, maxDistance)
			if err != nil || !distance {
				slog.Info("skipping url distance too long", slog.String("url", items[i].URL))
				continue
			}
			items[i].ItemName = Name
			items[i].Condition += " " + distStr
			items[i].URL = strings.Split(items[i].URL, "?ref")[0]
			items[i].Date = crawlDate
			items[i].Duration = 0
			items[i].AcceptsOffers = true
			retArr = append(retArr, items[i])
		}
	}
	return retArr, err
}

func GetCoordinates(Location string) (float64, float64, error) {
	base := "https://api.geoapify.com/v1/geocode/search?text="

	api := "&format=json&apiKey=" + os.Getenv("GEO_API_KEY")
	query := url.PathEscape(Location)
	url := base + query + api
	method := "GET"

	// ------------ get lat and long from description -----------
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		slog.Error("forming first request err", slog.Any("value", err))
		return 0, 0, err
	}
	res, err := client.Do(req)
	if err != nil {
		slog.Error("failed to execute first request",
			slog.Any("value", err))
		return 0, 0, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("failed to read first request",
			slog.Any("value", err))
		return 0, 0, err
	}

	var result GeocodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		slog.Error("failed to unmarshal into json first request",
			slog.Any("value", err))
		return 0, 0, err
	}
	if len(result.Results) == 0 {
		slog.Error("json result is empty in coordinate",
			slog.Any("value", err))
		return 0, 0, fmt.Errorf("no results found")
	}

	target := result.Results[0]
	return target.Lat, target.Lon, err
}

func ValidateDistance(location string, homeLat float64, homeLong float64, maxDistance int) (bool, string, error) {
	// --------------- get distance from api------------------
	api := "&format=json&apiKey=" + os.Getenv("GEO_API_KEY")
	url := "https://api.geoapify.com/v1/routematrix?" + api
	method := "POST"
	client := &http.Client{}

	targetLat, targetLong, err := GetCoordinates(location)
	if err != nil {
		slog.Error("failed to get coordinates",
			slog.Any("value", err))
		return false, "", err
	}
	t := coordinates{
		Location: [2]float64{targetLong, targetLat},
	}
	h := coordinates{
		Location: [2]float64{homeLong, homeLat},
	}
	reqBody := Body{
		Mode:    "drive",
		Sources: []coordinates{t},
		Targets: []coordinates{h},
		Units:   "imperial",
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		slog.Error("failed to convert to json",
			slog.Any("value", err))
		return false, "", err
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		slog.Error("failed to make new request to distance matrix",
			slog.Any("value", err))
		return false, "", err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		slog.Error("failed to execute second request",
			slog.Any("value", err))
		return false, "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		slog.Error("failed to read first request",
			slog.Any("value", err))
		return false, "", err
	}
	var d distanceRes
	if err := json.Unmarshal(body, &d); err != nil {
		slog.Error("failed to unmarshal second request",
			slog.Any("value", err))
		return false, "", err
	}
	if len(d.Sources_to_targets) == 0 {
		return false, "", fmt.Errorf("empty array returned from geo")
	} else if len(d.Sources_to_targets[0]) == 0 {
		return false, "", fmt.Errorf("empty array returned from geo")
	}
	Distance := d.Sources_to_targets[0][0].Distance
	Time := int(d.Sources_to_targets[0][0].Time)
	TimeMin := Time / 60

	if Distance < float64(maxDistance) {
		// format time and distance format to be displayed in the discord message
		retStr := fmt.Sprintf("%.1f miles, currently %d min ETA", Distance, TimeMin)
		slog.Info("formatted distance and time",
			slog.String("format", retStr))
		return true, retStr, err
	}
	return false, "", err
}
