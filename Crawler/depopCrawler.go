package crawler

import (
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/url"
	"time"

	types "priceTracker/Types"

	"github.com/gocolly/colly/v2"
)

func depopURLGenerator(Name string, price int) string {
	base := "https://www.depop.com/search/?q="
	Name = url.PathEscape(Name)
	Price := fmt.Sprintf("&_suggestion-type=recent&priceMax=%d", price)

	return base + Name + Price
}

// CrawlDepop retrieves clothing listings from Depop.
// It crawls the search results and visits each product page to verify
// CrawlDepop retrieves clothing listings from Depop.
// It crawls the search results and visits each product page to verify
// the item matches the expected patterns.
//
// Parameters:
//   - Name: the item name to search for
//   - Price: the maximum price for listings
//   - allRegexPatterns: compiled regex patterns for inclusion matching
//   - allSpecialWords: special character words for inclusion matching
//   - exclusionRegexes: compiled regex patterns for exclusion matching
//   - exclusionSpecialWords: special character words for exclusion matching
//
// Returns the listings and any error encountered.
func CrawlDepop(Name string,
	Price int,
	queries *ItemRegexInfo,
	attempts []*types.Attempt,
	proxy []string,
) ([]*types.EbayListing, error) {
	url := depopURLGenerator(Name, Price)
	var c *colly.Collector

	retArr := []*types.EbayListing{}
	visited := false
	if attempts == nil {
		attempts = []*types.Attempt{}
	}
	var proxyIndex int
	if len(proxy) != 0 {
		proxyIndex = rand.IntN(len(proxy))
		c = initCrawler(url, proxy[proxyIndex])
		slog.Info("proxy set for default crawler",
			slog.Any("proxyArr", proxy),
			slog.String("proxy url", proxy[proxyIndex]),
		)
	} else {
		c = initCrawler(url, "")
		slog.Info("proxy function set to nil")
	}
	slog.Info("logging depop url", slog.String("Url", url))
	c.OnHTML("ol[class^='styles_productGrid__'] li", func(e *colly.HTMLElement) {
		visited = true
		price, _ := formatPrice(e.ChildText("p.styles_price__H8qdh"))
		price = int(float64(price) * TaxRate)
		productURL := "https://depop.com" + e.ChildAttr("a", "href")
		if price > Price {
			slog.Debug("skipping depop item, price too high",
				slog.Int("Desired Price", Price),
				slog.Int("item price", price))
			return
		}
		proxyCopy := make([]string, len(proxy))
		copy(proxyCopy, proxy)
		listing, productAttempts, err := CrawlProductPage(productURL,
			queries, attempts, proxyCopy)
		attempts = append(attempts, productAttempts...)
		if err != nil {
			slog.Warn("could not visit product page")
			return
		}
		listing.ItemName = Name
		listing.Condition = Name
		listing.Price = price
		retArr = append(retArr, listing)
	})

	err := c.Visit(url)
	c.Wait()

	if err != nil || !visited {
		proxyVal := types.ProxyDisabled
		if len(proxy) != 0 {
			proxyVal = proxy[proxyIndex]
		}
		attempts = append(attempts, makeAttemptObject(types.CrawlerDepop,
			proxyVal, types.MethodColly,
			errOrMsg(err, "error object empty but not visited")))
		if len(proxy) != 0 {
			slog.Warn("depop proxy crawl failed, retrying without this proxy",
				slog.Any("Error", err),
				slog.String("proxy", proxy[proxyIndex]),
			)
			proxy = append(proxy[:proxyIndex], proxy[proxyIndex+1:]...)
			return CrawlDepop(Name, Price, queries, attempts, proxy)
		} else {
			slog.Error("depop crawl failed with no proxy")
			loggIncident(url, attempts, false)
			if err == nil {
				err = errors.New("Depop link not visited, might have been rate limited")
			}
			return retArr, err
		}
	}

	return retArr, nil
}

func CrawlProductPage(productURL string,
	queries *ItemRegexInfo,
	attempts []*types.Attempt,
	proxy []string,
) (*types.EbayListing, []*types.Attempt, error) {
	// Create NEW collector for product page
	var productCollector *colly.Collector
	approved := false
	visited := false
	condition := ""
	var proxyIndex int
	if len(proxy) != 0 {
		proxyIndex = rand.IntN(len(proxy))
		productCollector = initCrawler(productURL, proxy[proxyIndex])
		slog.Info("proxy set for default crawler",
			slog.Any("proxyArr", proxy),
			slog.String("proxy url", proxy[proxyIndex]),
		)
	} else {
		productCollector = initCrawler(productURL, "")
		slog.Info("proxy function set to nil")
	}

	// Handler for product page
	r := rand.IntN(30)
	r += 30
	time.Sleep(time.Duration(r) * time.Second)

	productCollector.OnHTML("p.styles_textWrapper__v3kxJ", func(pe *colly.HTMLElement) {
		visited = true
		condition = pe.Text
		if titleCorrectnessCheck(condition, queries) {
			approved = true
		}
	})

	// Visit product page synchronously
	err := productCollector.Visit(productURL)
	productCollector.Wait()

	Listing := &types.EbayListing{}
	if !visited || err != nil {
		slog.Warn("Depop product link not visited for",
			slog.String("url", productURL),
			slog.Any("error", err),
			slog.Bool("visited", visited),
		)
		if !visited && err == nil {
			err = errors.New("page not visited")
		}
		proxyVal := types.ProxyDisabled
		if len(proxy) != 0 {
			proxyVal = proxy[proxyIndex]
		}
		retAttempts := []*types.Attempt{makeAttemptObject(types.CrawlerDepop,
			proxyVal, types.MethodColly,
			errOrMsg(err, "product page not visited"))}
		if len(proxy) != 0 {
			slog.Warn("depop product page proxy failed, retrying without this proxy",
				slog.String("URL", productURL),
				slog.String("proxy", proxy[proxyIndex]),
			)
			proxy = append(proxy[:proxyIndex], proxy[proxyIndex+1:]...)
			proxyCopy := make([]string, len(proxy))
			copy(proxyCopy, proxy)
			return CrawlProductPage(productURL, queries, attempts, proxyCopy)
		}
		return Listing, retAttempts, err
	}
	// Now approved and condition are set
	if approved {
		Listing = &types.EbayListing{
			Title:         condition,
			URL:           productURL,
			Date:          time.Now(),
			Duration:      0,
			AcceptsOffers: true,
		}
		slog.Info("listing", slog.Any("depop listing information", Listing))
	} else {
		slog.Info("skipping depop item, title not matched",
			slog.String("URL", productURL),
		)
	}
	return Listing, attempts, err
}
