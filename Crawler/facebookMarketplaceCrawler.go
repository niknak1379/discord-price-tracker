package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	types "priceTracker/Types"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func GetSecondHandListings(Name string, Price int, homeLat float64, homeLong float64,
	maxDistance int, itemType string, LocationCode string,
) ([]types.EbayListing, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	var depop []types.EbayListing
	var err3 error
	if itemType == "Clothes" {
		Price = Price / 2
		depop, err3 = CrawlDepop(Name, Price)
	}
	fb, err2 := MarketPlaceCrawl(Name, Price, homeLat, homeLong, maxDistance, LocationCode)
	ebay, err := GetEbayListings(Name, Price)

	if err != nil || err2 != nil || err3 != nil {
		fmt.Println("errors from getting second hand listing", err, err2)
	}
	retArr := append(ebay, fb...)
	retArr = append(retArr, depop...)

	logger.Info("listing", slog.Any("Listing Values", retArr))
	return retArr, errors.Join(err, err2)
}

func FacebookURLGenerator(Name string, Price int, LocationCode string) string {
	baseURL := "https://www.facebook.com/marketplace/107711145919004/search"
	priceQuery := fmt.Sprintf("?maxPrice=%d", Price)
	query := "&query=" + url.PathEscape(Name) + "&exact=false"
	return baseURL + priceQuery + query
}

// JS loaded cannot use colly for this
func MarketPlaceCrawl(Name string, desiredPrice int, homeLat float64, homeLong float64,
	maxDistance int, LocationCode string,
) ([]types.EbayListing, error) {
	crawlDate := time.Now()
	url := FacebookURLGenerator(Name, desiredPrice, LocationCode)
	fmt.Println("crawling ", url)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("log-level", "3"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, timeoutCancel := context.WithTimeout(ctx, 60*time.Second) // Increased timeout
	defer timeoutCancel()
	var first []byte
	var second []byte
	var items []types.EbayListing
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(10*time.Second),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.FullScreenshot(&first, 90), // 90 = JPEG quality
		chromedp.Click("div.xdg88n9.x10l6tqk.x1tk7jg1.x1vjfegm"),
		chromedp.Sleep(3*time.Second),
		chromedp.FullScreenshot(&second, 90), // 90 = JPEG quality
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

		// Price: parseInt((e.querySelector('span.x193iq5w.xeuugli.x13faqbe.x1vvkbs.xlh3980.xvmahel.x1n0sxbx.x1lliihq.x1s928wv.xhkezso.x1gmr53x.x1cpjm7i.x1fgarty.x1943h6x.x4zkp8e.x676frb.x1lkfr7t.x1lbecb7.x1s688f.xzsf02u')?.innerText || '0' ).replace('$', '').replaceAll(',', '')),
		//                                        x193iq5w xeuugli x13faqbe x1vvkbs xlh3980 xvmahel x1n0sxbx x1lliihq x1s928wv xhkezso x1gmr53x x1cpjm7i x1fgarty x1943h6x x4zkp8e x3x7a5m x1lkfr7t x1lbecb7 x1s688f xzsf02u
	)

	os.WriteFile("first.png", first, 0644)
	os.WriteFile("second.png", second, 0644)

	var retArr []types.EbayListing
	if err != nil {
		fmt.Println(err)
		return retArr, err
	} else if len(items) == 0 {
		return retArr, errors.New("no items returned from facebook, check screenshots for sanity check")
	}
	// <------------------ sanitize the list ------------>
	for i := range items {
		if titleCorrectnessCheck(items[i].Title, Name) && items[i].Price != 0 {
			distance, distStr, err := ValidateDistance(items[i].Condition, homeLat,
				homeLong, maxDistance)
			if err != nil || !distance {
				fmt.Println("skipping url distance too long", items[i].URL)
				continue
			}
			items[i].ItemName = Name
			items[i].Condition += " " + distStr
			items[i].URL = strings.Split(items[i].URL, "?ref")[0]
			items[i].Date = crawlDate
			fmt.Println("appending facebook listing for", items[i].Title, items[i].Condition)
			retArr = append(retArr, items[i])
		} else {
			fmt.Println("skipping title, criteria not met", items[i].Title)
		}
	}
	return retArr, err
}

func GetCoordinates(Location string) (float64, float64, error) {
	base := "https://api.geoapify.com/v1/geocode/search?text="
	// logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	api := "&format=json&apiKey=" + os.Getenv("GEO_API_KEY")
	query := url.PathEscape(Location)
	url := base + query + api
	method := "GET"

	// ------------ get lat and long from description -----------
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Println("forming first request err", err)
		return 0, 0, err
	}
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("faild first request", err)
		return 0, 0, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("reading first body", err)
		return 0, 0, err
	}

	// fmt.Println(string(body))
	var result GeocodeResponse
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Println("unmarshaling into result object for first request", err)
		return 0, 0, err
	}
	// logger.Info("listing", slog.Any("Listing marshalled respone from first req", result))
	if len(result.Results) == 0 {
		fmt.Println("result is empty")
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
		fmt.Println("converting to json", err)
		return false, "", err
	}
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Println("forming second request", err)
		return false, "", err
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println("second request err", err)
		return false, "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("reading second request err", err)
		return false, "", err
	}
	// logger.Info("req", slog.Any("req", string(jsonBody)))
	// fmt.Println("printing second body", string(body))
	var d distanceRes
	if err := json.Unmarshal(body, &d); err != nil {
		fmt.Println("unmarshaling into result object for first request", err)
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
		fmt.Println("formatted distance", retStr)
		return true, retStr, err
	}
	return false, "", err
}
