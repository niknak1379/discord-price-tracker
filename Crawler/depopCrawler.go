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

func CrawlDepop(Name string, Price int) ([]types.EbayListing, error) {
	url := depopURLGenerator(Name, Price)
	c := initCrawler()

	crawlDate := time.Now()
	retArr := []types.EbayListing{}
	visited := false
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

		// Create NEW collector for product page
		productCollector := initCrawler()
		approved := false
		condition := ""

		// Handler for product page
		r := rand.IntN(30)
		r += r + 30
		time.Sleep(time.Duration(r) * time.Second)

		productCollector.OnHTML("p.styles_textWrapper__v3kxJ", func(pe *colly.HTMLElement) {
			condition = pe.Text
			if titleCorrectnessCheck(condition, Name) {
				approved = true
			}
		})

		// Visit product page synchronously
		productCollector.Visit(productURL)
		productCollector.Wait()

		// Now approved and condition are set
		if approved && price < Price {
			Listing := types.EbayListing{
				ItemName:      Name,
				Title:         condition,
				Price:         price,
				Condition:     Name,
				URL:           productURL,
				Date:          crawlDate,
				Duration:      0,
				AcceptsOffers: true,
			}
			slog.Info("listing", slog.Any("depop listing information", Listing))
			retArr = append(retArr, Listing)
		} else {
			slog.Info("skipping depop item, title not matched or price too high",
				slog.String("URL", url))
		}
	})

	err := c.Visit(url)
	c.Wait()

	if err != nil || !visited {
		if err == nil {
			err = errors.New("Depop link not visited, might have been rate limited")
		}
		return retArr, err
	}

	return retArr, nil
}
