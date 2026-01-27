package crawler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	// "log/slog"

	// "os"

	types "priceTracker/Types"

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
	location := "&_stpos=90274&_fcid=1"
	return baseURL + url.PathEscape(Name) + usedQuery + priceQuery + noAuction + location
}

// returns a map of urls and prices + shipping cost
// it returns an error on items that are local pickup only
// since they dont have a shipping fee div
func GetEbayListings(Name string, desiredPrice int, Proxy bool) ([]types.EbayListing, error) {
	url := ConstructEbaySearchURL(Name, desiredPrice)

	slog.Info(url, slog.Bool("proxy", Proxy))
	var listingArr []types.EbayListing
	crawlDate := time.Now()
	visited := false
	c := initCrawler()
	if !Proxy {
		c.SetProxyFunc(nil)
	}
	c.OnHTML("ul.srp-results > li", func(e *colly.HTMLElement) {
		visited = true
		title := e.ChildText(".s-card__title span.primary")

		// check to see if listing is viable
		if !titleCorrectnessCheck(title, Name) {
			slog.Info("skipping title criteria not met", slog.String("Title", title))
			return
		}
		condition := e.ChildText("div.s-card__subtitle")

		// first one is price, second one is wether its bid or normal "or best offer" GetEbayListings
		// thid is delivery price +$12.00 delivery in 2-4 days
		var basePrice, shippingCost int
		var err error
		var acceptsOffers bool
		e.ForEachWithBreak("div.s-card__attribute-row", func(i int, child *colly.HTMLElement) bool {
			switch i {
			case 0:
				// get base price
				basePrice, err = formatPrice(child.Text)
				basePrice = int(float64(basePrice) * TaxRate)
			case 1:
				// skip bids, no need to add them to the return bid array
				if strings.Contains(child.Text, "or Best Offer") {
					acceptsOffers = true
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
			slog.Warn("price 0 something is wrong for", slog.Any("Error", err),
				slog.Int("baseprice", basePrice), slog.String("URL", link))
			return
		} else if basePrice+shippingCost >= desiredPrice {
			slog.Info("price too high skipping title", slog.String("Title", title))
			return
		}

		listing := types.EbayListing{
			ItemName: Name,
			Price:    shippingCost + basePrice,
			// it has metadata from search after url, this leans it up
			URL:           strings.Split(link, "?_skw")[0],
			Title:         title,
			AcceptsOffers: acceptsOffers,
			Condition:     condition,
			Date:          crawlDate,
			Duration:      0,
		}
		slog.Info("listing", slog.Any("ebay listing information", listing))
		listingArr = append(listingArr, listing)
	})
	err := c.Visit(url)
	c.Wait()
	if err != nil || !visited {
		if !Proxy {
			slog.Warn("Colly failed even without proxy triggering chromeDP")
			listingArr, err = EbayFailover(url, desiredPrice, Name)
			return listingArr, err
		}
		slog.Warn("ebay failed, redoing request without proxy")
		listingArr, err = GetEbayListings(Name, desiredPrice, false)
		return listingArr, err
	}
	return listingArr, err
}

func EbayFailover(url string, desiredPrice int, Name string) ([]types.EbayListing, error) {
	crawlDate := time.Now()
	slog.Info("chromedp failover for ebay", slog.String("URL", url))
	ctx, cancel := NewChromedpContext(90 * time.Second)
	defer cancel()

	var first []byte
	var second []byte
	var items []types.EbayListing
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		StealthActions(),
		chromedp.Sleep(10*time.Second),
		chromedp.FullScreenshot(&first, 70),
		chromedp.Sleep(3*time.Second),
		chromedp.FullScreenshot(&second, 70),
		chromedp.Evaluate(`
		Array.from(document.querySelectorAll('ul.srp-results > li')).map(e => {
				const rows = e.querySelectorAll('div.s-card__attribute-row');
				let basePrice = 0;
				let shippingCost = 0;
				let AcceptsOffer = false
				
				// Format price function (converted from Go)
				const formatPrice = (priceStr) => {
						if (!priceStr) return 0;
						let ret = priceStr.replace(/\$/g, '');
						ret = ret.replace(/,/g, '');
						ret = ret.trim();
						ret = ret.split('.')[0];
						return parseInt(ret) || 0;
				};
				
				for (let i = 0; i < Math.min(3, rows.length); i++) {
						if (i === 0) {
								basePrice = formatPrice(rows[i].innerText);
						}
						if (i === 1 && rows[i].innerText.includes('or Best Offer')) {
								AcceptsOffer = true;
						}
						if (i === 2) {
								if (rows[i].innerText.includes('Free delivery')) {
										shippingCost = 0;
								} else {
										shippingCost = formatPrice(rows[i].innerText);
								}
						}
				}
				
				return {
						Title: e.querySelector('.s-card__title span.primary')?.innerText || '',
						Condition: e.querySelector('div.s-card__subtitle')?.innerText || '',
						URL: e.querySelector('a.s-card__link')?.href || '',
						AcceptsOffer: AcceptsOffer,
						Price: shippingCost + basePrice
				};
		}).filter(item => item !== null)
		`, &items),
	)
	var retArr []types.EbayListing

	if err != nil {
		fileErr1 := os.WriteFile("ebayFirst.png", first, 0o644)
		fileErr2 := os.WriteFile("ebaySecond.png", second, 0o644)
		slog.Error("Error in ebay failover", slog.Any("error value", err),
			slog.Any("file error 1", fileErr1), slog.Any("file error 2", fileErr2))
		return retArr, errors.Join(err, errors.New("Problem in Ebay chromeDP Failover"))
	} else if len(items) == 0 {
		return retArr, errors.New("no items returned from Ebay chromeDP, check screenshots for sanity check")
	}
	slog.Info("Ebay Failover returned Items, its fine for now")
	// <------------------ sanitize the list ------------>
	for i := range items {
		if titleCorrectnessCheck(items[i].Title, Name) && items[i].Price != 0 &&
			items[i].Price < desiredPrice {
			items[i].ItemName = Name
			items[i].URL = strings.Split(items[i].URL, "?_skw")[0]
			items[i].Price = int(float64(items[i].Price) * TaxRate)
			items[i].Date = crawlDate
			items[i].Duration = 0
			retArr = append(retArr, items[i])
		}
	}
	return retArr, err
}

// checks the title to make sure the name is in the title and
// no unwanted returned results by ebay
func titleCorrectnessCheck(listingTitle string, itemName string) bool {
	words := strings.Fields(strings.ToLower(itemName))
	listingTitle = strings.ToLower(listingTitle)
	replacer := strings.NewReplacer(
		".", " ",
		"'", " ",
		"’", " ",
	)
	listingTitle = replacer.Replace(listingTitle)

	// short designators like x, xt or numbers get lost, so add spaces
	// around them ----- changed my mind ill do it for all of them
	// ----- a lot of models still get mixed up especially for monitors
	for _, word := range words {
		pattern := `\b` + regexp.QuoteMeta(word) + `\b`
		matched, _ := regexp.MatchString(pattern, listingTitle)
		if !matched {
			return false // Word not found
		}
	}
	// exludes titles that have these key words
	excludeArr := [15]string{
		`\bfor parts`, `\bbroken`, `\baccessories\b`,
		`\bbox only`, `\bempty box`, `\bcable\b`, `\bdongle\b`,
		`\bkids\b`, `\bjunior\b`, `read`, `\bstand\b`, `\badapter\b`, `\bdefective`,
		`damage`, `problem`,
	}
	for _, excludeQuery := range excludeArr {
		query, _ := regexp.MatchString(excludeQuery, listingTitle)
		if query {
			return false
		}
	}
	return true
}

// i dont need this anymore but ill keep it just in case
// i was being dumb and didnt see the start of the url
// was there and i didnt have to crawl the link itself
func getCanonicalURL(c *colly.Collector, url string) string {
	retURL := url
	parsed := false
	c.OnHTML("link[rel='canonical']", func(e *colly.HTMLElement) {
		retURL = e.Attr("href")
		parsed = true
	})
	err := c.Visit(url)
	if err != nil || !parsed {
		// already have a url if it fails its fine
		return retURL
	}
	return retURL
}
