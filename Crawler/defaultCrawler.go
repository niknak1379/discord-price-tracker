package crawler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	logger "priceTracker/Logger"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

func initCrawler() *colly.Collector {
	// --------------------------- initiaize scrapper headers and settings ------- //
	var c *colly.Collector
	c = colly.NewCollector(
		colly.MaxDepth(1),
		colly.AllowURLRevisit(),
	)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*ebay.*",
		Delay:       1 * time.Minute,
		RandomDelay: 3 * time.Minute,
	})

	c.SetRequestTimeout(30 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       2 * time.Second, 
		RandomDelay: 1 * time.Second, 
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

	c.OnError(func(r *colly.Response, err error) {
		s := fmt.Sprintf("Error scraping %s: %v", r.Request.URL, err)
		logger.Logger.Error(s)
	})
	return c
}

func GetPrice(uri string, querySelector string) (int, error) {
	var err error
	res := 0
	crawled := false
	logger.Logger.Info("logging url", slog.String("URI", uri))
	c := initCrawler()
	c.OnHTML(querySelector, func(h *colly.HTMLElement) {
		crawled = true
		res, err = formatPrice(h.Text)
		c.OnHTMLDetach(querySelector)
	})
	err = c.Visit(uri)

	c.Wait()
	if !crawled {
		err = errors.New("could not crawl, html element does not exist")
	}
	if err != nil {
		logger.Logger.Error("error in getting price in crawler, triggering Chrome failover", 
			slog.Any("Error", err))
		res, err := ChromeDPFailover(uri, querySelector)
		return res, err
	}
	return res, err
}

func ChromeDPFailover(url string, selector string) (int, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("log-level", "3"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, timeoutCancel := context.WithTimeout(ctx, 60*time.Second)
	defer timeoutCancel()

	var priceText string
	var htmlContent []byte

	// Split into separate runs to debug where it fails
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(10*time.Second),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.FullScreenshot(&htmlContent, 90),
		chromedp.Text(selector, &priceText, chromedp.ByQuery),
	)
	if err != nil {
		os.WriteFile("failoverSS.png", htmlContent, 0644)
		return 0, fmt.Errorf("selector '%s' not found for url %s, %w", selector, url, err)
	}

	logger.Logger.Info("ChromeDP found Selector")
	
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
	if strings.Contains(url, "amazon") {
		c.OnHTML("img#landingImage", func(e *colly.HTMLElement) {
			imgURL = e.Attr("src")
			visited = true
		})
	} else if strings.Contains(url, "bestbuy") {
		c.OnHTML("div.VJYXIrZT4D0Zj6vQ img", func(e *colly.HTMLElement) {
			imgURL = e.Attr("src")
			visited = true
		})
	}else {
		c.OnHTML("meta[property='og:image']", func(e *colly.HTMLElement) {
			imgURL = e.Attr("content")
			visited = true
		})
	}
	err := c.Visit(url)
	if err != nil || !visited {
		logger.Logger.Warn("could not get Open Graph picture", slog.Any("ERROR: ", err), slog.Any("Visited: ", visited))
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
