// Package crawler provides web scraping functionality for price tracking.
// It supports multiple sources including eBay, Facebook Marketplace, and Depop.
// The package uses colly for standard HTTP scraping and chromedp for JavaScript-heavy pages.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	logger "priceTracker/Logger"
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
	c := initCrawler()

	if attempts == nil {
		attempts = []*types.Attempt{}
	}
	var proxyIndex int
	if len(proxy) != 0 {
		proxyIndex = rand.IntN(len(proxy))
		c.SetProxy(proxy[proxyIndex])
		slog.Info("proxy set for colly ebay crawler",
			slog.Any("proxyArr", proxy),
			slog.String("proxy url", proxy[proxyIndex]),
		)
	} else {
		slog.Info("proxy function set to nil for ebay colly")
	}
	EbayHTMLProcessorCallback(c, Name, desiredPrice, queries, listingArr, bidArr, &visited)
	err := c.Visit(url)
	c.Wait()
	if err != nil || !visited {
		if len(proxy) == 0 {
			slog.Warn("Colly failed even without proxy triggering chromeDP without proxy")
			attempts = append(attempts, &types.Attempt{
				Crawler:   types.CrawlerEbay,
				Proxy:     types.ProxyDisabled,
				Method:    types.MethodColly,
				Timestamp: time.Now(),
				Error: func(err error) string {
					if err != nil {
						return err.Error()
					} else {
						return errors.New("error object empty but not visited").Error()
					}
				}(err),
			})
			listingArr, bidArr, err = EbayFailover(url, desiredPrice, Name, proxy, 0, queries, attempts)
			return listingArr, bidArr, err
		} else {
			slog.Warn("ebay failed, redoing request with chromedp with proxy")
			attempts = append(attempts, &types.Attempt{
				Crawler:   types.CrawlerEbay,
				Proxy:     proxy[proxyIndex],
				Method:    types.MethodColly,
				Timestamp: time.Now(),
				Error: func(err error) string {
					if err != nil {
						return err.Error()
					} else {
						return errors.New("error object empty but not visited").Error()
					}
				}(err),
			})
			listingArr, bidArr, err = EbayFailover(url, desiredPrice, Name, proxy, proxyIndex, queries, attempts)
			return listingArr, bidArr, err
		}
	}
	if len(attempts) != 0 {
		logger.IncidentChannel <- types.Incident{
			StartTime: time.Now(),
			URL:       url,
			Domain:    "ebay",
			Attempts:  attempts,
			Resolved:  true,
		}
	}
	return listingArr, bidArr, err
}

func EbayHTMLProcessorCallback(c *colly.Collector,
	Name string,
	desiredPrice int,
	queries *ItemRegexInfo,
	listingArr []*types.EbayListing,
	bidArr []*types.EbayBids,
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
			bidArr = append(bidArr, &listing)
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
			listingArr = append(listingArr, &listing)
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
	crawlDate := time.Now()
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

	type EbayItem struct {
		Title        string
		Condition    string
		URL          string
		Price        int
		AcceptsOffer bool
		IsBid        bool
		Bids         int
		EndTimeText  string
	}

	var items []*EbayItem
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
			let taxRate = 1.1;
			let shippingCost = 0;
			let AcceptsOffer = false;
			let isBid = false;
			let bids = 0;
			let endTimeText = '';
			
			const formatPrice = (priceStr) => {
					if (!priceStr) return 0;
					let ret = priceStr.replace(/\$/g, '');
					ret = ret.replace(/,/g, '');
					ret = ret.trim();
					ret = ret.split('.')[0];
					return parseInt(ret) || 0;
			};
			
			if (rows.length > 1 && rows[1].innerText.includes('bids')) {
					isBid = true;
			}
			
			for (let i = 0; i < Math.min(3, rows.length); i++) {
					if (i === 0) {
							basePrice = formatPrice(rows[i].innerText);
					}
					if (i === 1) {
							if (isBid) {
								endTimeText = rows[i].innerText;  // Store raw text
							} else if (rows[i].innerText.includes('or Best Offer')) {
								AcceptsOffer = true;
							}
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
					Condition: e.querySelector('div.s-card__subtitle:last-child')?.innerText || '',
					URL: e.querySelector('a.s-card__link')?.href || '',
					AcceptsOffer: AcceptsOffer,
					Price: shippingCost + basePrice * taxRate,
					IsBid: isBid,
					Bids: bids,
					EndTimeText: endTimeText
			};
	}).filter(item => item !== null)		`, &items),
	)
	cancel()
	var retListingArr []*types.EbayListing
	var retBidArr []*types.EbayBids

	if err != nil || len(items) == 0 {
		if len(proxy) != 0 {
			slog.Warn("Proxy ebay chrome failover failed, calling nonproxy default",
				slog.Any("error", err))
			attempts = append(attempts, &types.Attempt{
				Crawler:   types.CrawlerEbay,
				Proxy:     proxy[proxyIndexUsed],
				Method:    types.MethodChromeDP,
				Timestamp: time.Now(),
				Error: func(err error) string {
					if err != nil {
						return err.Error()
					} else {
						return errors.New("error object empty but not visited").Error()
					}
				}(err),
			})
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
			attempts = append(attempts, &types.Attempt{
				Crawler:   types.CrawlerEbay,
				Proxy:     types.ProxyDisabled,
				Method:    types.MethodChromeDP,
				Timestamp: time.Now(),
				Error: func(err error) string {
					if err != nil {
						return err.Error()
					} else {
						return errors.New("error object empty but not visited").Error()
					}
				}(err),
			})
			logger.IncidentChannel <- types.Incident{
				StartTime: time.Now(),
				URL:       url,
				Domain:    "ebay",
				Attempts:  attempts,
				Resolved:  false,
			}
			return retListingArr, retBidArr, errors.Join(err, errors.New("Problem in Ebay chromeDP Failover"))
		}
	}
	slog.Info("Ebay Failover returned Items, its fine for now")

	// Sanitize the list
	for i := range items {
		if !titleCorrectnessCheck(items[i].Title, queries) {
			continue
		}

		if items[i].Price == 0 {
			continue
		}

		if items[i].Price >= desiredPrice || items[i].Price <= int(float64(desiredPrice)*float64(0.25)) {
			continue
		}

		if items[i].IsBid {
			var bids int
			var endTime time.Time
			bids, endTime = ProcessBidRawString(items[i].EndTimeText, time.Now())
			if time.Until(endTime) > 24*time.Hour {
				continue
			}
			bidListing := types.EbayBids{
				ItemName:  Name,
				Price:     int(float64(items[i].Price) * TaxRate),
				URL:       strings.Split(items[i].URL, "?_skw")[0],
				Title:     items[i].Title,
				Condition: items[i].Condition,
				EndDate:   endTime,
				Bids:      bids,
			}
			retBidArr = append(retBidArr, &bidListing)
		} else {
			// Handle normal listing
			listing := types.EbayListing{
				ItemName:      Name,
				Price:         int(float64(items[i].Price) * TaxRate),
				URL:           strings.Split(items[i].URL, "?_skw")[0],
				Title:         items[i].Title,
				Condition:     items[i].Condition,
				AcceptsOffers: items[i].AcceptsOffer,
				Date:          crawlDate,
				Duration:      0,
			}
			retListingArr = append(retListingArr, &listing)
		}
	}
	logger.IncidentChannel <- types.Incident{
		StartTime: time.Now(),
		URL:       url,
		Domain:    "ebay",
		Attempts:  attempts,
		Resolved:  true,
	}
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
	c := initCrawler()

	var listingArr []*types.EbayListing
	var bidArr []*types.EbayBids
	visited := false
	EbayHTMLProcessorCallback(c, itemName, desiredPrice, &queries, listingArr, bidArr, &visited)
	err := c.Visit(ts.URL)
	if !visited && err != nil {
		slog.Error("Error was nil, but ebay not visited")
		err = errors.New("ebay page not visited")
	}
	c.Wait()
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
