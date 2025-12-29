package crawler

import (
	"fmt"
	"log"
	"log/slog"
	"net/url"
	"os"
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

type EbayListing struct {
	Price     int
	URL       string
	Title     string
	Condition string
}

// returns a map of urls and prices + shipping cost
func GetEbayListings(url string, Name string) []EbayListing {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	log.Println("visiting ebay url ", url)
	var listingArr []EbayListing
	visited := false
	c := initCrawler()
	c.OnHTML("ul.srp-results > li", func(e *colly.HTMLElement) {
		visited = true
		title := e.ChildText(".s-card__title span.primary")

		// check to see if listing is viable
		if !titleCorrectnessCheck(title, Name) {
			return
		}
		link := e.ChildAttr("a.s-card__link", "href")
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
		// skip item if any errors are met
		if basePrice == 0 || err != nil {
			log.Print("price 0 something is wrong for", err, basePrice, link)
			return
		}

		listing := EbayListing{
			Price:     shippingCost + basePrice,
			URL:       link,
			Title:     title,
			Condition: condition,
		}
		logger.Info("listing", slog.Any("Listing Values", listing))
	})
	err := c.Visit(url)
	c.Wait()
	if err != nil || !visited {
		log.Println("error in getting ebay listings", err, visited)
		return listingArr
	}
	return listingArr
}

// checks the title to make sure the name is in the title and
// no unwanted returned results by ebay
func titleCorrectnessCheck(listingTitle string, itemName string) bool {
	words := strings.Fields(strings.ToLower(itemName))
	listingTitle = strings.ToLower(listingTitle)

	// regex query
	pattern := strings.Join(words, ".*")
	pattern = ".*" + pattern
	matched, _ := regexp.MatchString(pattern, listingTitle)
	hasParts, _ := regexp.MatchString(`for\s+parts`, listingTitle)

	log.Println("printing from regex", listingTitle, itemName, matched)
	return matched && !hasParts
}
