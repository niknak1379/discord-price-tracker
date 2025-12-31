package crawler

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	// "log/slog"
	"net/url"
	// "os"
	types "priceTracker/Types"
	"regexp"
	"strings"

	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
)

func ConstructEbaySearchURL(Name string, newPrice int) string {
	baseURL := "https://www.ebay.com/sch/i.html?_nkw="
	usedQuery := "&LH_ItemCondition=3000|2020|2010|1500"
	priceQuery := fmt.Sprintf("&_udhi=%d&rt=nc", newPrice)
	return baseURL + url.PathEscape(Name) + usedQuery + priceQuery
}

// returns a map of urls and prices + shipping cost
func GetEbayListings(Name string, desiredPrice int) ([]types.EbayListing, error) {
	url := ConstructEbaySearchURL(Name, desiredPrice)
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
		if basePrice == 0 || err != nil {
			log.Print("price 0 something is wrong for", err, basePrice+shippingCost, link)
			return
		}
		if basePrice+shippingCost >= desiredPrice {
			fmt.Println("price too high skipping")
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

func FacebookURLGenerator(Name string, Price int) string {
	baseURL := "https://www.facebook.com/marketplace/107711145919004/search"
	priceQuery := fmt.Sprintf("?maxPrice=%d", Price)
	query := "&query=" + url.PathEscape(Name) + "&exact=false"
	return baseURL + priceQuery + query
}

// JS loaded cannot use colly for this
func MarketPlaceCrawl(Name string, desiredPrice int) ([]types.EbayListing, error) {
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
	}
	// <------------------ sanitize the list ------------>
	for _, item := range items {
		if titleCorrectnessCheck(item.Title, Name) && item.Price != 0 {
			retArr = append(retArr, item)
		}
	}
	return retArr, err
}

func GetSecondHandListings(Name string, Price int) ([]types.EbayListing, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	fb, err2 := MarketPlaceCrawl(Name, Price)
	ebay, err := GetEbayListings(Name, Price)
	if err != nil || err2 != nil {
		fmt.Println("errors from getting second hand listing", err, err2)
	}
	retArr := append(ebay, fb...)
	logger.Info("listing", slog.Any("Listing Values", retArr))
	return retArr, err
}
