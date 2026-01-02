package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	// "log/slog"
	"net/http"
	"net/url"

	// "os"
	types "priceTracker/Types"
	"regexp"
	"strings"

	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
)

type GeocodeResponse struct {
	Results []Location `json:"results"`
}

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

func ConstructEbaySearchURL(Name string, newPrice int) string {
	baseURL := "https://www.ebay.com/sch/i.html?_nkw="
	usedQuery := "&LH_ItemCondition=3000|2020|2010|1500"
	priceQuery := fmt.Sprintf("&_udhi=%d&rt=nc", newPrice)
	noAuction := "&LH_BIN=1"
	return baseURL + url.PathEscape(Name) + usedQuery + priceQuery + noAuction
}

// returns a map of urls and prices + shipping cost
func GetEbayListings(Name string, desiredPrice int) ([]types.EbayListing, error) {
	url := ConstructEbaySearchURL(Name, desiredPrice)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log.Println("visiting ebay url ", url)
	var listingArr []types.EbayListing
	visited := false
	c := initCrawler()
	c.OnHTML("ul.srp-results > li", func(e *colly.HTMLElement) {
		visited = true
		title := e.ChildText(".s-card__title span.primary")

		// check to see if listing is viable
		if !titleCorrectnessCheck(title, Name) {
			fmt.Println("skipping title criteria not met", title)
			return
		}
		condition := e.ChildText("div.s-card__subtitle")

		// first one is price, second one is wether its bid or normal "or best offer" GetEbayListings
		// thid is delivery price +$12.00 delivery in 2-4 days
		var basePrice, shippingCost int
		var err error
		e.ForEachWithBreak("div.s-card__attribute-row", func(i int, child *colly.HTMLElement) bool {
			switch i {
			case 0:
				// get base price
				basePrice, err = formatPrice(child.Text)
			case 1:
				// skip bids, no need to add them to the return bid array
				if strings.Contains(child.Text, "bid") {
					return true
				}
			case 2:
				// get shipping price
				if strings.Contains(child.Text, "Free delivery") {
					shippingCost = 0
				} else {
					shippingCost, err = formatPrice(child.Text)
				}
			default:
				return false
			}
			return true
		})
		link := e.ChildAttr("a.s-card__link", "href")
		// skip item if any errors are met
		if basePrice == 0 || err != nil {
			log.Print("price 0 something is wrong for", err, basePrice+shippingCost, link)
			return
		}
		if basePrice+shippingCost >= desiredPrice {
			fmt.Println("price too high skipping")
			return
		}

		listing := types.EbayListing{
			Price: shippingCost + basePrice,
			// it has metadata from search after url, this leans it up
			URL:       strings.Split(link, "?_skw")[0],
			Title:     title,
			Condition: condition,
		}
		logger.Info("listing", slog.Any("ebay listing information", listing))
		listingArr = append(listingArr, listing)
	})
	err := c.Visit(url)
	c.Wait()
	if err != nil || !visited {
		log.Println("error in getting ebay listings, no items were visited or an err occured", err, visited)
		if !visited && err == nil {
			err = errors.New("no items were visited from ebay.com")
		}
		return listingArr, err
	}
	return listingArr, err
}

// checks the title to make sure the name is in the title and
// no unwanted returned results by ebay
func titleCorrectnessCheck(listingTitle string, itemName string) bool {
	words := strings.Fields(strings.ToLower(itemName))
	listingTitle = strings.ToLower(listingTitle)

	// short designators like x, xt or numbers get lost, so add spaces
	// around them ----- changed my mind ill do it for all of them
	// ----- a lot of models still get mixed up especially for monitors
	var patterns []string
	for _, word := range words {
		patterns = append(patterns, `\b`+regexp.QuoteMeta(word)+`\b`)
	}

	pattern := strings.Join(patterns, ".*")
	pattern = ".*" + pattern

	matched, _ := regexp.MatchString(pattern, listingTitle)
	// exludes titles that have these key words
	excludeArr := [7]string{
		`\bfor\s+parts\b`, `\bbroken\b`, `\baccessories\b`,
		`\bbox only\b`, `\bempty box\b`, `\bcable\b`, `\bdongle\b`,
	}
	for _, excludeQuery := range excludeArr {
		query, _ := regexp.MatchString(excludeQuery, listingTitle)
		if query {
			return false
		}
	}
	return matched
}

func getCanonicalURL(c *colly.Collector, url string) string {
	retURL := url
	parsed := false
	fmt.Println("getting canonical url for", url)
	c.OnHTML("link[rel='canonical']", func(e *colly.HTMLElement) {
		retURL = e.Attr("href")
		fmt.Println("got new link", retURL)
		parsed = true
	})
	err := c.Visit(url)
	if err != nil || !parsed {
		// already have a url if it fails its fine
		fmt.Println("error or not parsed in canonical", err, parsed)
		return retURL
	}
	return retURL
}

func FacebookURLGenerator(Name string, Price int) string {
	baseURL := "https://www.facebook.com/marketplace/107711145919004/search"
	priceQuery := fmt.Sprintf("?maxPrice=%d", Price)
	query := "&query=" + url.PathEscape(Name) + "&exact=false"
	return baseURL + priceQuery + query
}

// JS loaded cannot use colly for this
func MarketPlaceCrawl(Name string, desiredPrice int, Channel types.Channel) ([]types.EbayListing, error) {
	url := FacebookURLGenerator(Name, desiredPrice)
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
			distance, distStr, err := ValidateDistance(items[i].Condition, Channel)
			if err != nil || !distance {
				fmt.Println("skipping url distance too long", items[i].URL)
				continue
			}
			items[i].Condition += " " + distStr
			items[i].URL = strings.Split(items[i].URL, "?ref")[0]
			fmt.Println("appending facebook listing for", items[i].Title, items[i].Condition)
			retArr = append(retArr, items[i])
		} else {
			fmt.Println("skipping title, criteria not met", items[i].Title)
		}
	}
	return retArr, err
}

func GetSecondHandListings(Name string, Price int, Channel types.Channel) ([]types.EbayListing, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	fb, err2 := MarketPlaceCrawl(Name, Price, Channel)
	ebay, err := GetEbayListings(Name, Price)
	if err != nil || err2 != nil {
		fmt.Println("errors from getting second hand listing", err, err2)
	}
	retArr := append(ebay, fb...)
	logger.Info("listing", slog.Any("Listing Values", retArr))
	return retArr, errors.Join(err, err2)
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

func ValidateDistance(location string, homeLat float64, homeLong float64) (bool, string, error) {
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

	if Distance < 30 {
		// format time and distance format to be displayed in the discord message
		retStr := fmt.Sprintf("%.1f miles, currently %d min ETA", Distance, TimeMin)
		fmt.Println("formatted distance", retStr)
		return true, retStr, err
	}
	return false, "", err
}
