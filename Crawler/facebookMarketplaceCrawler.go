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

	logger "priceTracker/Logger"
	Proxy "priceTracker/Proxy"
	types "priceTracker/Types"

	"github.com/chromedp/chromedp"
)

// GeocodeResponse represents the response from the geocoding API.
type GeocodeResponse struct {
	Results []Location `json:"results"`
}

// Location represents a geographic coordinate.
type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}
type Body struct {
	Mode    string        `json:"mode"`
	Sources []coordinates `json:"sources"`
	Targets []coordinates `json:"targets"`
	Units   string        `json:"units"`
}
type coordinates struct {
	Location [2]float64 `json:"location"`
}
type dist struct {
	Distance float64 `json:"distance"`
	Time     float64 `json:"time"`
}
type distanceRes struct {
	Sources_to_targets [][]dist `json:"sources_to_targets"`
}

// GetSecondHandListings aggregates listings from multiple second-hand sources.
// It searches eBay, Facebook Marketplace (if enabled), and Depop (for clothing items).
// Results are deduplicated by URL and filtered by price range.
//
// Parameters:
//   - Names: list of item names to search for
//   - Price: the maximum price for listings
//   - homeLat: home latitude for distance calculation
//   - homeLong: home longitude for distance calculation
//   - maxDistance: maximum distance from home in miles
//   - itemType: the item category (Tech or Clothes)
//   - LocationCode: the Facebook Marketplace location code
//   - facebookCrawl: whether to crawl Facebook Marketplace
//   - exclusionQueries: patterns to exclude from results
//
// Returns the listings, bids, and any error encountered.
func GetSecondHandListings(Names []string, Price int, homeLat float64, homeLong float64,
	maxDistance int, itemType string, LocationCode string, facebookCrawl bool,
	exclusionQueries []string,
	proxyArr []string,
) ([]*types.EbayListing, []*types.EbayBids, error, error, error) {
	retListingArr := []*types.EbayListing{}
	retBidArr := []*types.EbayBids{}
	queries := InitTitleRegex(Names, exclusionQueries)
	var ebayErr, fbErr, depopErr error
	for _, Name := range Names {

		var depop, fb []*types.EbayListing
		var err3, err2 error
		if itemType == "Clothes" {
			proxyCopy := make([]string, len(proxyArr))
			copy(proxyCopy, proxyArr)
			// im bargen hunting here close only for those less than half price
			depop, err3 = CrawlDepop(Name, Price/2, queries, nil, proxyCopy)
		}
		if facebookCrawl {
			proxyCopy := make([]string, len(proxyArr))
			copy(proxyCopy, proxyArr)
			fb, err2 = MarketPlaceCrawl(Name, Price, homeLat, homeLong,
				maxDistance, LocationCode, proxyCopy, queries, nil)
		}
		proxyCopy := make([]string, len(proxyArr))
		copy(proxyCopy, proxyArr)
		ebay, bids, err4 := GetEbayListings(Name, Price, proxyCopy,
			queries, nil)

		ebayErr = errors.Join(ebayErr, err4)
		fbErr = errors.Join(fbErr, err2)
		depopErr = errors.Join(depopErr, err3)
		if err4 != nil {
			slog.Warn("ebay crawl failed", slog.Any("error", err4))
		}
		if err2 != nil {
			slog.Warn("Facebook Marketplace crawl failed", slog.Any("error", err2))
		}
		if err3 != nil {
			slog.Warn("Depop crawl failed", slog.Any("error", err3))
		}

		retListingArr = append(retListingArr, ebay...)
		retListingArr = append(retListingArr, fb...)
		retListingArr = append(retListingArr, depop...)
		retBidArr = append(retBidArr, bids...)
		t := rand.IntN(240) + 60
		time.Sleep(time.Duration(t) * time.Second)
	}
	dedupMap := make(map[string]*types.EbayListing)
	for _, Listing := range retListingArr {
		dedupMap[Listing.URL] = Listing
	}
	dedupArr := []*types.EbayListing{}
	for _, Listing := range dedupMap {
		dedupArr = append(dedupArr, Listing)
	}
	dedupBidMap := make(map[string]*types.EbayBids)
	for _, Listing := range retBidArr {
		dedupBidMap[Listing.URL] = Listing
	}
	dedupBidArr := []*types.EbayBids{}
	for _, Listing := range dedupBidMap {
		dedupBidArr = append(dedupBidArr, Listing)
	}
	return dedupArr, dedupBidArr, ebayErr, fbErr, depopErr
}

// FacebookURLGenerator builds a Facebook Marketplace search URL.
//
// Parameters:
//   - Name: the item name to search for
//   - Price: the maximum price for listings
//   - LocationCode: the Facebook Marketplace location code
//
// Returns the constructed Facebook Marketplace search URL.
func FacebookURLGenerator(Name string, Price int, LocationCode string) string {
	baseURL := "https://www.facebook.com/marketplace/" + LocationCode + "/search"
	priceQuery := fmt.Sprintf("?maxPrice=%d", Price)
	query := "&query=" + url.PathEscape(Name) + "&exact=false"
	return baseURL + priceQuery + query
}

// MarketPlaceCrawl retrieves listings from Facebook Marketplace using chromedp.
// Facebook Marketplace requires JavaScript rendering, so standard HTTP scraping won't work.
// Listings are filtered by distance from the specified location.
//
// Parameters:
//   - Name: the item name to search for
//   - desiredPrice: the maximum price for listings
//   - homeLat: home latitude for distance calculation
//   - homeLong: home longitude for distance calculation
//   - maxDistance: maximum distance from home in miles
//   - LocationCode: the Facebook Marketplace location code
//   - proxy: whether to use a proxy for the request
//   - allRegexPatterns: compiled regex patterns for inclusion matching
//   - allSpecialWords: special character words for inclusion matching
//   - exclusionRegexes: compiled regex patterns for exclusion matching
//   - exclusionSpecialWords: special character words for exclusion matching
//
// Returns the listings and any error encountered.
func MarketPlaceCrawl(Name string, desiredPrice int, homeLat, homeLong float64,
	maxDistance int, LocationCode string,
	proxy []string,
	queries *ItemRegexInfo,
	attempts []*types.Attempt,
) ([]*types.EbayListing, error) {
	crawlDate := time.Now()
	url := FacebookURLGenerator(Name, desiredPrice, LocationCode)
	slog.Info("crawling facebook marketplace URL", slog.String("URL", url))
	var ctx context.Context
	var cancel context.CancelFunc
	var proxyIndexUsed int
	if len(proxy) != 0 {
		proxyIndexUsed = rand.IntN(len(proxy))
	}
	ctx, cancel = NewChromedpContext(90, &proxy, proxyIndexUsed)
	if attempts == nil {
		attempts = []*types.Attempt{}
	}

	var first []byte
	var HTMLContent string
	var items []types.EbayListing
	err := chromedp.Run(ctx,
		StealthActions(url),
		chromedp.Navigate(url),
		chromedp.Sleep(time.Duration(rand.IntN(10)+15)*time.Second),
		chromedp.Evaluate(`document.querySelector('div.xdg88n9.x10l6tqk.x1tk7jg1.x1vjfegm')?.click()`, nil),
		chromedp.OuterHTML("body", &HTMLContent),
		chromedp.FullScreenshot(&first, 70),
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`
		Array.from(document.querySelectorAll("div[data-virtualized='false']")).map(e => ({
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
	logger.LogFileChannel <- makeCrawlFilesObject(Name, types.CrawlerFacebook, HTMLContent, first)
	cancel()
	var retArr []*types.EbayListing
	if len(items) == 0 || err != nil {
		if len(proxy) != 0 {
			time.Sleep(5 * time.Second)
			attempts = append(attempts, makeAttemptObject(types.CrawlerFacebook,
				types.ProxyEnabled, types.MethodChromeDP,
				errOrMsg(err, "error object empty but not visited")))
			proxy = append(proxy[:proxyIndexUsed], proxy[proxyIndexUsed+1:]...)
			slog.Warn("facebook proxy failed, triggering no proxy crawl",
				slog.Any("Error", err),
				slog.Int("ItemArr length", len(items)),
				slog.Any("proxy", proxy),
			)
			return MarketPlaceCrawl(Name, desiredPrice, homeLat, homeLong, maxDistance, LocationCode,
				proxy, queries, attempts)
		} else {
			slog.Error("Error in marketplace", slog.Any("error value", err))
			attempts = append(attempts, makeAttemptObject(types.CrawlerFacebook,
				types.ProxyDisabled, types.MethodChromeDP,
				errOrMsg(err, "error object empty but not visited")))
			loggIncident(url, attempts, false)
			ErrMsg := "empty error object but not visited"
			if err != nil {
				ErrMsg = err.Error()
			}
			err = types.MakeError(types.ErrFacebook, ErrMsg, url)
			return retArr, err
		}
	}
	// <------------------ sanitize the list ------------>
	for i := range items {
		if titleCorrectnessCheck(items[i].Title, queries) &&
			items[i].Price != 0 &&
			items[i].Price < desiredPrice &&
			items[i].Price >= int(float64(desiredPrice)*float64(0.25)) {
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
			retArr = append(retArr, &items[i])
		}
	}
	if len(attempts) != 0 {
		loggIncident(url, attempts, true)
	}
	if len(proxy) != 0 {
		Proxy.ProxySuccessChannel <- proxy[proxyIndexUsed]
	}
	return retArr, err
}

// GetCoordinates converts a location string (e.g., "Los Angeles, CA") to latitude and longitude.
// Uses the Geoapify geocoding API.
//
// Parameters:
//   - Location: the location string to geocode
//
// Returns the latitude, longitude, and any error encountered.
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

// ValidateDistance checks if a location is within the specified maximum distance from home coordinates.
//
// Parameters:
//   - location: the location string to check
//   - homeLat: home latitude
//   - homeLong: home longitude
//   - maxDistance: maximum distance in miles
//
// Returns true if within range, the distance string for display, and any error.
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
