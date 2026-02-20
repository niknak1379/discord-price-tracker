// Package crawler provides web scraping functionality for price tracking.
// It supports multiple sources including eBay, Facebook Marketplace, and Depop.
// The package uses colly for standard HTTP scraping and chromedp for JavaScript-heavy pages.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	types "priceTracker/Types"

	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
)

// ConstructEbaySearchURL builds an eBay search URL with filters for used items.
// The price range is set to 25%-100% of the desired price to find comparable used listings.
//
// Parameters:
//   - Name: the item name to search for
//   - newPrice: the desired price to base the search range on
//
// Returns the constructed eBay search URL.
func ConstructEbaySearchURL(Name string, newPrice int) string {
	baseURL := "https://www.ebay.com/sch/i.html?_nkw="
	usedQuery := "&LH_ItemCondition=3000|2030|2020|2010|2000|1500|1000"
	priceQuery := fmt.Sprintf("_udlo=%d&rt=nc&_udhi=%d", int(float64(newPrice)*float64(0.25)), newPrice)
	noAuction := "&LH_ALL=1"
	location := "&_stpos=90274&_fcid=1"
	return baseURL + url.PathEscape(Name) + usedQuery + priceQuery + noAuction + location
}

// GetEbayListings retrieves eBay listings matching the search criteria.
// It first attempts to scrape using colly, then falls back to chromedp if needed.
// Listings are filtered by price range (25%-100% of desired price) and validated
// against inclusion/exclusion patterns.
//
// Parameters:
//   - Name: the item name to search for
//   - desiredPrice: the maximum price for listings
//   - Proxy: whether to use a proxy for the request
//   - allRegexPatterns: compiled regex patterns for inclusion matching
//   - allSpecialWords: special character words for inclusion matching
//   - exclusionRegexes: compiled regex patterns for exclusion matching
//   - exclusionSpecialWords: special character words for exclusion matching
//
// Returns the listings, bids, and any error encountered.
func GetEbayListings(Name string,
	desiredPrice int,
	proxy []string,
	queries *ItemRegexInfo,
	attempts []*types.Attempt,
) ([]*types.EbayListing, []*types.EbayBids, error) {
	url := ConstructEbaySearchURL(Name, desiredPrice)

	slog.Info("crawling ebay url", slog.String("URL", url))
	var listingArr []*types.EbayListing
	var bidArr []*types.EbayBids
	visited := false
	var c *colly.Collector
	var proxyIndex int
	c, proxyIndex = initCrawler(url, &proxy)
	if attempts == nil {
		attempts = []*types.Attempt{}
	}
	EbayHTMLProcessorCallback(c, Name, desiredPrice, queries, &listingArr, &bidArr, &visited)
	err := c.Visit(url)
	c.Wait()
	if err != nil || !visited {
		if len(proxy) == 0 {
			slog.Warn("Colly failed even without proxy triggering chromeDP without proxy")
			attempts = append(attempts, makeAttemptObject(types.CrawlerEbay,
				types.ProxyDisabled, types.MethodColly,
				errOrMsg(err, "error object empty but not visited")))
			listingArr, bidArr, err = EbayFailover(url, desiredPrice, Name, proxy, 0, queries, attempts)
			return listingArr, bidArr, err
		} else {
			slog.Warn("ebay failed, redoing request with chromedp with proxy")
			attempts = append(attempts, makeAttemptObject(types.CrawlerEbay,
				proxy[proxyIndex], types.MethodColly,
				errOrMsg(err, "error object empty but not visited")))
			listingArr, bidArr, err = EbayFailover(url, desiredPrice, Name, proxy, proxyIndex, queries, attempts)
			return listingArr, bidArr, err
		}
	}
	if len(attempts) != 0 {
		loggIncident(url, attempts, true)
	}
	return listingArr, bidArr, err
}

func EbayHTMLProcessorCallback(c *colly.Collector,
	Name string,
	desiredPrice int,
	queries *ItemRegexInfo,
	listingArr *[]*types.EbayListing,
	bidArr *[]*types.EbayBids,
	visited *bool,
) {
	crawlDate := time.Now()
	c.OnHTML("ul.srp-results > li", func(e *colly.HTMLElement) {
		*visited = true
		title := e.ChildText(".s-card__title span.primary")

		// check to see if listing is viable
		if !titleCorrectnessCheck(title, queries) {
			slog.Info("skipping title criteria not met", slog.String("Title", title))
			return
		}
		condition := e.ChildText("div.s-card__subtitle:last-child")
		// checks wether element is a bid or not
		isBid := false
		e.ForEachWithBreak("div.s-card__attribute-row", func(i int, child *colly.HTMLElement) bool {
			if i == 1 {
				if strings.Contains(child.Text, "bid") {
					isBid = true
				}
				return false // Stop after index 1
			}
			return true
		})

		// check wether bid or not
		if isBid {
			slog.Info("Bid Listing found", slog.String("Title", title),
				slog.String("elementText", e.Text))
			var basePrice, shippingCost int
			var err error
			var bids int
			var endTime time.Time
			e.ForEachWithBreak("div.s-card__attribute-row", func(i int, child *colly.HTMLElement) bool {
				switch i {
				case 0:
					// get base price
					basePrice, err = formatPrice(child.Text)
					basePrice = int(float64(basePrice) * TaxRate)
				case 1:
					// index 1 is where auction end and how many bids data is held
					bids, endTime = ProcessBidRawString(child.Text, time.Now())
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
			} else if basePrice+shippingCost >= desiredPrice ||
				basePrice+shippingCost <= int(float64(desiredPrice)*float64(0.25)) ||
				time.Until(endTime) > 24*time.Hour {
				slog.Info("price too high or end date too far, skipping bid",
					slog.String("Title", title),
				)
				return
			}

			listing := types.EbayBids{
				ItemName:  Name,
				Price:     shippingCost + basePrice,
				URL:       strings.Split(link, "?_skw")[0],
				Title:     title,
				Condition: condition,
				EndDate:   endTime,
				Bids:      bids,
			}
			slog.Info("bid", slog.Any("ebay listing information", listing))
			*bidArr = append(*bidArr, &listing)
		} else {

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
			} else if basePrice+shippingCost >= desiredPrice ||
				basePrice <= int(float64(desiredPrice)*float64(0.25)) {
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
			*listingArr = append(*listingArr, &listing)
		}
	})
}

// EbayFailover attempts to retrieve eBay listings using chromedp when colly fails.
// It provides a headless browser fallback for JavaScript-heavy pages.
// Screenshots are saved on failure for debugging purposes.
//
// Parameters:
//   - url: the eBay search URL to scrape
//   - desiredPrice: the maximum price for listings
//   - Name: the item name for logging
//   - proxy: whether to use a proxy for the request
//   - allRegexPatterns: compiled regex patterns for inclusion matching
//   - allSpecialWords: special character words for inclusion matching
//   - exclusionRegexes: compiled regex patterns for exclusion matching
//   - exclusionSpecialWords: special character words for exclusion matching
//
// Returns the listings, bids, and any error encountered.
func EbayFailover(url string, desiredPrice int, Name string, proxy []string,
	proxyIndexUsed int,
	queries *ItemRegexInfo,
	attempts []*types.Attempt,
) (
	[]*types.EbayListing, []*types.EbayBids, error,
) {
	time.Sleep(5 * time.Second)
	slog.Info("chromedp failover for ebay", slog.String("URL", url))
	var ctx context.Context
	var cancel context.CancelFunc
	if len(proxy) != 0 {
		proxyURL := proxy[proxyIndexUsed]
		ctx, cancel = NewChromedpContext(90*time.Second,
			chromedp.ProxyServer(proxyURL),
		)
		slog.Info("proxy set for ebay chromeDP",
			slog.String("proxy url", proxy[proxyIndexUsed]),
		)
	} else {
		slog.Info("proxy set to nil for ebay chromeDP")
		ctx, cancel = NewChromedpContext(90 * time.Second)
	}

	var first []byte
	var second []byte
	var rawHTML string

	err := chromedp.Run(ctx,
		StealthActions(url),
		chromedp.Navigate(url),
		chromedp.Sleep(10*time.Second),
		chromedp.FullScreenshot(&first, 70),
		chromedp.Sleep(7*time.Second),
		chromedp.FullScreenshot(&second, 70),
		chromedp.OuterHTML("html", &rawHTML),
	)
	cancel()
	var retListingArr []*types.EbayListing
	var retBidArr []*types.EbayBids
	retListingArr, retBidArr, err = ParseChromedpHTML(rawHTML, desiredPrice, Name, *queries)
	if err != nil {
		if len(proxy) != 0 {
			slog.Warn("Proxy ebay chrome failover failed, calling nonproxy default",
				slog.Any("error", err))
			attempts = append(attempts, makeAttemptObject(types.CrawlerEbay,
				proxy[proxyIndexUsed], types.MethodChromeDP,
				errOrMsg(err, "error object empty but not visited")))
			proxy = append(proxy[:proxyIndexUsed], proxy[proxyIndexUsed+1:]...)
			slog.Warn("ChromDP proxy failed, triggering default crawler without this proxy",
				slog.Any("error", err),
				slog.Any("new Proxy Arr", proxy),
			)
			return GetEbayListings(Name, desiredPrice, proxy, queries, attempts)
		} else {
			fileErr1 := os.WriteFile("logs/ebayFirst.png", first, 0o644)
			fileErr2 := os.WriteFile("logs/ebaySecond.png", second, 0o644)
			slog.Error("Error in ebay failover", slog.Any("error value", err),
				slog.Any("file error 1", fileErr1), slog.Any("file error 2", fileErr2))
			attempts = append(attempts, makeAttemptObject(types.CrawlerEbay,
				types.ProxyDisabled, types.MethodChromeDP,
				errOrMsg(err, "error object empty but not visited")))
			loggIncident(url, attempts, false)
			return retListingArr, retBidArr, errors.Join(err, errors.New("Problem in Ebay chromeDP Failover"))
		}
	}
	slog.Info("Ebay Failover returned Items, its fine for now")

	loggIncident(url, attempts, true)
	return retListingArr, retBidArr, err
}

// ParseChromedpHTML - new function to parse chromedp output with colly
func ParseChromedpHTML(html string,
	desiredPrice int,
	itemName string,
	queries ItemRegexInfo,
) ([]*types.EbayListing, []*types.EbayBids, error) {
	// Create test server with the chromedp HTML
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()
	var c *colly.Collector
	c, _ = initCrawler("", &[]string{})

	var listingArr []*types.EbayListing
	var bidArr []*types.EbayBids
	visited := false
	EbayHTMLProcessorCallback(c, itemName, desiredPrice, &queries, &listingArr, &bidArr, &visited)
	err := c.Visit(ts.URL)
	c.Wait()
	if !visited && err == nil {
		slog.Error("Error was nil, but ebay not visited")
		err = errors.New("ebay page not visited")
	}
	return listingArr, bidArr, err
}

// im gonna silent error this for now, it just returns
// everything 0 if it errs on sth
func ProcessBidRawString(rawString string, currTime time.Time) (int, time.Time) {
	// 34 bids · Time left2d 23h left
	var bids int
	var err error
	var endTime time.Time
	if rawString == "" {
		return 0, time.Time{}
	}
	bidInfo := strings.Split(rawString, "·")
	// 34
	bidStr := strings.Split(bidInfo[0], " ")[0]
	bids, err = strconv.Atoi(bidStr)
	if err != nil {
		slog.Warn("Cant process bid number",
			slog.Any("bidInfo", bidInfo),
			slog.String("bidStr", bidStr),
			slog.Any("error", err),
		)
	}
	bidInfo[1] = strings.ReplaceAll(bidInfo[1], " Time left", "")
	timeLeftStr := strings.Split(bidInfo[1], "left")
	if strings.Contains(timeLeftStr[0], "d") {
		// 2d 16h left
		timeLeftStr = strings.Split(timeLeftStr[0], " ")
		day, err := strconv.Atoi(strings.Split(timeLeftStr[0], "d")[0])
		if err != nil {
			slog.Warn("Cant process bid Day",

				slog.Any("bidInfo", bidInfo),
				slog.String("bidStr", bidStr),
				slog.Any("timeLeftStr", timeLeftStr),
				slog.Any("error", err),
			)
		}
		hour := 0
		if len(timeLeftStr) > 1 {
			hour, err = strconv.Atoi(strings.Split(timeLeftStr[1], "h")[0])
			if err != nil {
				slog.Warn("Cant process bid hour",
					slog.Any("bidInfo", bidInfo),
					slog.String("bidStr", bidStr),
					slog.Any("timeLeftStr", timeLeftStr),
					slog.Any("error", err),
				)
			}
		}
		endTime = currTime.Add(
			time.Duration(day)*time.Hour*24 +
				time.Duration(hour)*time.Hour,
		)

	} else if strings.Contains(timeLeftStr[0], "m") &&
		!strings.Contains(timeLeftStr[0], "h") {
		timeLeftStr = strings.Split(timeLeftStr[0], " ")
		minute, err := strconv.Atoi(strings.Split(timeLeftStr[0], "m")[0])
		if err != nil {
			slog.Warn("Cant process bid Minute",

				slog.Any("bidInfo", bidInfo),
				slog.String("bidStr", bidStr),
				slog.Any("timeLeftStr", timeLeftStr),
				slog.Any("error", err),
			)
		}
		second := 0
		if len(timeLeftStr) > 1 {
			second, err = strconv.Atoi(strings.Split(timeLeftStr[1], "s")[0])
			if err != nil {
				slog.Warn("Cant process bid second",
					slog.Any("bidInfo", bidInfo),
					slog.String("bidStr", bidStr),
					slog.Any("timeLeftStr", timeLeftStr),
					slog.Any("error", err),
				)
			}
		}
		endTime = currTime.Add(
			time.Duration(minute)*time.Minute +
				time.Duration(second)*time.Second,
		)
	} else {
		timeLeftStr = strings.Split(timeLeftStr[0], " ")
		hour, err := strconv.Atoi(strings.Split(timeLeftStr[0], "h")[0])
		if err != nil {
			slog.Warn("Cant process bid hour",
				slog.Any("bidInfo", bidInfo),
				slog.String("bidStr", bidStr),
				slog.Any("timeLeftStr", timeLeftStr),
				slog.Any("error", err))
		}
		minute := 0
		if len(timeLeftStr) > 1 {
			minute, err = strconv.Atoi(strings.Split(timeLeftStr[1], "m")[0])
			if err != nil {
				slog.Warn("Cant process bid min",
					slog.Any("bidInfo", bidInfo),
					slog.String("bidStr", bidStr),
					slog.Any("timeLeftStr", timeLeftStr),
					slog.Any("error", err),
				)
			}
		}
		endTime = currTime.Add(
			time.Duration(hour)*time.Hour +
				time.Duration(minute)*time.Minute,
		)
	}
	return bids, endTime
}
