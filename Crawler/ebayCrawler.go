package crawler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	// "log/slog"

	// "os"
	logger "priceTracker/Logger"
	types "priceTracker/Types"

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
// it returns an error on items that are local pickup only
// since they dont have a shipping fee div
func GetEbayListings(Name string, desiredPrice int) ([]types.EbayListing, error) {
	url := ConstructEbaySearchURL(Name, desiredPrice)

	logger.Logger.Info(url)
	var listingArr []types.EbayListing
	crawlDate := time.Now()
	visited := false
	c := initCrawler()
	c.OnHTML("ul.srp-results > li", func(e *colly.HTMLElement) {
		visited = true
		title := e.ChildText(".s-card__title span.primary")

		// check to see if listing is viable
		if !titleCorrectnessCheck(title, Name) {
			logger.Logger.Info("skipping title criteria not met", slog.String("Title", title))
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
			logger.Logger.Warn("price 0 something is wrong for", slog.Any("Error", err), 
				slog.Int("baseprice", basePrice), slog.String("URL", link))
			return
		} else if basePrice+shippingCost >= desiredPrice {
			logger.Logger.Info("price too high skipping title", slog.String("Title", title))
			return
		}

		listing := types.EbayListing{
			ItemName: Name,
			Price:    shippingCost + basePrice,
			// it has metadata from search after url, this leans it up
			URL:       strings.Split(link, "?_skw")[0],
			Title:     title,
			Condition: condition,
			Date:      crawlDate,
			Duration:  0,
		}
		logger.Logger.Info("listing", slog.Any("ebay listing information", listing))
		listingArr = append(listingArr, listing)
	})
	err := c.Visit(url)
	c.Wait()
	if err != nil || !visited {
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
	excludeArr := [13]string{
		`\bfor parts\b`, `\bbroken\b`, `\baccessories\b`,
		`\bbox\b`, `\bempty box\b`, `\bcable\b`, `\bdongle\b`,
		`\bkids\b`, `\bjunior\b`, `\bread\b`, `\bstand\b`, `\badapter\b`, `\bdefective\b`,
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
