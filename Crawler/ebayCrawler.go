package crawler

import (
	"fmt"
	"log"

	// "log/slog"
	"net/url"
	// "os"
	types "priceTracker/Types"
	"regexp"
	"strings"

	"github.com/gocolly/colly/v2"
)

func ConstructEbaySearchURL(Name string, newPrice int) string {
	baseURL := "https://www.ebay.com/sch/i.html?_nkw="
	usedQuery := "&LH_ItemCondition=3000|2020|2010|1500"
	priceQuery := fmt.Sprintf("&_udhi=%d&rt=nc", newPrice)
	return baseURL + url.PathEscape(Name) + usedQuery + priceQuery
}

// returns a map of urls and prices + shipping cost
func GetEbayListings(url string, Name string, desiredPrice int) ([]types.EbayListing, error) {
	// logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log.Println("visiting ebay url ", url)
	var listingArr []types.EbayListing
	visited := false
	c := initCrawler()
	c.OnHTML("ul.srp-results > li", func(e *colly.HTMLElement) {
		visited = true
		title := e.ChildText(".s-card__title span.primary")

		// check to see if listing is viable
		if !titleCorrectnessCheck(title, Name) {
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
		if basePrice == 0 || err != nil || basePrice+shippingCost >= desiredPrice {
			log.Print("price 0 something is wrong for", err, basePrice+shippingCost, link)
			return
		}

		link = getCanonicalURL(c, link)
		listing := types.EbayListing{
			Price:     shippingCost + basePrice,
			URL:       link,
			Title:     title,
			Condition: condition,
		}
		// logger.Info("listing", slog.Any("Listing Values", listing))
		listingArr = append(listingArr, listing)
	})
	err := c.Visit(url)
	c.Wait()
	if err != nil || !visited {
		log.Println("error in getting ebay listings", err, visited)
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
	// around them
	var patterns []string
	for _, word := range words {
		if len(word) < 4 {
			patterns = append(patterns, `\b`+regexp.QuoteMeta(word)+`\b`)
		} else {
			patterns = append(patterns, regexp.QuoteMeta(word))
		}
	}

	pattern := strings.Join(patterns, ".*")
	pattern = ".*" + pattern

	matched, _ := regexp.MatchString(pattern, listingTitle)
	// exludes titles that have for parts
	hasParts, _ := regexp.MatchString(`\bfor\s+parts\b`, listingTitle)
	hasBroken, _ := regexp.MatchString(`\bbroken\b`, listingTitle)
	hasAccessories, _ := regexp.MatchString(`\baccessories\b`, listingTitle)
	return matched && !hasParts && !hasBroken && !hasAccessories
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
		fmt.Println(err)
		return retURL
	}
	return retURL
}
