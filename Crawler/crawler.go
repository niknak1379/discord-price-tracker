package crawler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

func initCrawler() *colly.Collector {
	log.Println("initalizing crawler")
	// --------------------------- initiaize scrapper headers and settings ------- //
	var c *colly.Collector
	c = colly.NewCollector(
		colly.MaxDepth(1),
		colly.AllowURLRevisit(),
	)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*ebay.*",
		Delay:       1 * time.Minute,
		RandomDelay: 3 * time.Minute, // Random 2-5 seconds
	})

	c.SetRequestTimeout(30 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       2 * time.Second, // Wait 2 seconds between requests
		RandomDelay: 1 * time.Second, // Add random delay (1-3 seconds total)
	})
	extensions.RandomUserAgent(c)
	c.OnRequest(func(r *colly.Request) {
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
		r.Headers.Set("DNT", "1")
		r.Headers.Set("Connection", "keep-alive")
		r.Headers.Set("Upgrade-Insecure-Requests", "1")
		r.Headers.Set("Sec-Fetch-Dest", "document")
		r.Headers.Set("Sec-Fetch-Mode", "navigate")
		r.Headers.Set("Sec-Fetch-Site", "cross-site")
		r.Headers.Set("Referer", "https://www.google.com/")
		r.Headers.Set("Accept-Encoding", "gzip, deflate")
	})
	c.WithTransport(&http.Transport{
		DisableCompression: false,
	})

	c.OnResponse(func(r *colly.Response) {
		log.Printf("Content-Encoding: %s", r.Headers.Get("Content-Encoding"))
		log.Printf("Status Code: %d", r.StatusCode)
		log.Printf("Content-Type: %s", r.Headers.Get("Content-Type"))
		log.Printf("Body length: %d", len(r.Body))
		// log.Printf("Body: %s", r.Body)
	})
	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Error scraping %s: %v", r.Request.URL, err)
	})
	return c
}

func GetPrice(uri string, querySelector string) (int, error) {
	var err error
	res := 0
	crawled := false
	log.Println("logging url", uri, querySelector)
	c := initCrawler()
	c.OnHTML(querySelector, func(h *colly.HTMLElement) {
		log.Println(querySelector, h.Text)
		crawled = true
		res, err = formatPrice(h.Text) // runs on all matched queries, i just want the first one
		c.OnHTMLDetach(querySelector)
	})
	err = c.Visit(uri)

	c.Wait()
	if !crawled {
		err = errors.New("could not crawl, html element does not exist")
	}
	if err != nil {
		log.Println("error in getting price in crawler, triggering Chrome failover", err)
		res, err := chromeDPFailover(uri, querySelector)
		log.Println("price", res, err)
		return res, err
	}
	log.Println("price", res, err)
	return res, err
}

func chromeDPFailover(url, selector string) (int, error) {
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

	var priceText string
	var htmlContent string

	// Split into separate runs to debug where it fails
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(3*time.Second),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	)

	// Save HTML even if there's an error later
	os.WriteFile("debug-page.html", []byte(htmlContent), 0644)
	log.Printf("Saved debug-page.html (%d bytes)", len(htmlContent))

	if err != nil {
		return 0, fmt.Errorf("page load failed: %w", err)
	}

	// Now try to get price
	err = chromedp.Run(ctx,
		chromedp.Sleep(2*time.Second),
		chromedp.Text(selector, &priceText, chromedp.ByQuery),
	)
	if err != nil {
		return 0, fmt.Errorf("selector '%s' not found: %w", selector, err)
	}

	// Parse price
	priceText = strings.ReplaceAll(priceText, "$", "")
	priceText = strings.ReplaceAll(priceText, ",", "")
	priceText = strings.TrimSpace(priceText)

	if strings.Contains(priceText, ".") {
		priceText = strings.Split(priceText, ".")[0]
	}

	price, err := strconv.Atoi(priceText)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price '%s': %w", priceText, err)
	}

	return price, nil
}

func GetOpenGraphPic(url string) string {
	c := initCrawler()
	visited := false
	imgURL := ""
	log.Println("logging url", url)
	if strings.Contains(url, "amazon.com") {
		c.OnHTML("img#landingImage", func(e *colly.HTMLElement) {
			imgURL = e.Attr("src")
			fmt.Println("OG Image:", imgURL)
			visited = true
		})
	} else {
		c.OnHTML("meta[property='og:image']", func(e *colly.HTMLElement) {
			imgURL = e.Attr("content")
			fmt.Println("OG Image:", imgURL)
			visited = true
		})
	}
	err := c.Visit(url)
	if err != nil || !visited {
		fmt.Println("could not get Open Graph picture", err, visited)
		return ""
	}
	c.Wait()
	return imgURL
}

func formatPrice(priceStr string) (int, error) {
	ret := strings.ReplaceAll(priceStr, "$", "")
	ret = strings.ReplaceAll(ret, ",", "")
	ret = strings.TrimSpace(ret)
	ret = strings.Split(ret, ".")[0]
	res, err := strconv.Atoi(ret)
	return res, err
}
