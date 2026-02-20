package crawler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"time"

	types "priceTracker/Types"

	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
)

// GetPrice retrieves the price from a product page using the specified CSS selector.
// It first attempts with colly, then falls back to chromedp if needed.
// GetPrice retrieves the price from a product page using the specified CSS selector.
// It first attempts with colly, then falls back to chromedp if needed.
//
// Parameters:
//   - uri: the URL to scrape
//   - querySelector: the CSS selector for extracting the price
//   - proxy: whether to use a proxy for the request
//
// Returns the price and any error encountered.
func GetPrice(uri string, querySelector string, proxy []string, attempts []*types.Attempt) (int, error) {
	var err, priceErr error
	var proxyIndex int
	var collyHTML string
	res := 0
	crawled := false
	slog.Info("logging url", slog.String("URI", uri))
	c := initCrawler(uri)
	if attempts == nil {
		attempts = []*types.Attempt{}
	}
	if len(proxy) != 0 {
		proxyIndex = rand.IntN(len(proxy))
		c.SetProxy(proxy[proxyIndex])
		slog.Info("proxy set for default crawler",
			slog.Any("proxyArr", proxy),
			slog.String("proxy url", proxy[proxyIndex]),
		)
	} else {
		slog.Info("proxy function set to nil")
	}
	c.OnHTML("body", func(h *colly.HTMLElement) {
		collyHTML, _ = h.DOM.Html()
	})
	DefaultParserCallback(c, querySelector, &crawled, &res, &priceErr)
	err = c.Visit(uri)
	c.Wait()
	if !crawled {
		err = errors.New("Error in default crawler: could not crawl, html element does not exist")
	}
	if err != nil || priceErr != nil {
		var res int
		var err2 error
		os.WriteFile("logs/collyHTML.html", []byte(collyHTML), 0o644)
		if len(proxy) != 0 {
			slog.Warn("error in getting price in crawler, triggering proxy chrome",
				slog.Any("Error", err), slog.Any("PriceErr", priceErr))
			attempts = append(attempts, makeAttemptObject(types.CrawlerDefault, proxy[proxyIndex], types.MethodColly, firstErrorMsg(err, priceErr, "unknown error")))
			res, err2 = ChromeDPFailover(uri, querySelector, proxy, proxyIndex, attempts)
			return res, err2
		} else {
			slog.Warn("no proxy default crawler failed, triggering chromeDPFailover no proxy",
				slog.Any("Error", err2),
				slog.Int("Price", res),
			)
			attempts = append(attempts, makeAttemptObject(types.CrawlerDefault, types.ProxyDisabled, types.MethodColly, firstErrorMsg(err, priceErr, "unknown error")))
			res, err2 = ChromeDPFailover(uri, querySelector, proxy, proxyIndex, attempts)
			return res, err2
		}
	}
	if len(attempts) != 0 {
		loggIncident(uri, attempts, true)
	}
	return int(float64(res) * TaxRate), err
}

func DefaultParserCallback(c *colly.Collector, querySelector string, crawled *bool, price *int, priceErr *error) {
	c.OnHTML(querySelector, func(h *colly.HTMLElement) {
		if *crawled {
			return
		}
		slog.Info("default crawler call back function being called")
		*crawled = true
		*price, *priceErr = formatPrice(h.Text)
		c.OnHTMLDetach(querySelector)
	})
}

// ParseChromedpHTML - new function to parse chromedp output with colly
func ParseDefaultChromedpHTML(html string,
	querySelector string,
) (int, error) {
	// Create test server with the chromedp HTML
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()
	c := initCrawler("")
	crawled := false
	price := 0
	var priceErr error

	DefaultParserCallback(c, querySelector, &crawled, &price, &priceErr)
	err := c.Visit(ts.URL)
	c.Wait()
	if !crawled && err == nil {
		slog.Error("Error was nil, but ebay not visited")
		err = errors.New("ebay page not visited")
	} else if priceErr != nil {
		err = priceErr
	}
	return price, err
}

// ChromeDPFailover attempts to retrieve a price using chromedp when colly fails.
// It uses a headless browser for JavaScript-heavy pages.
// Screenshots and HTML are saved on failure for debugging.
//
// Parameters:
//   - url: the URL to scrape
//   - selector: the CSS selector for extracting the price
//   - proxy: whether to use a proxy for the request
//
// Returns the price and any error encountered.
func ChromeDPFailover(url string, selector string, proxy []string, proxyIndexUsed int, attempts []*types.Attempt) (int, error) {
	slog.Warn("ChromeDP Triggered for default crawler",
		slog.String("URL", url), slog.String("Selector", selector),
	)
	time.Sleep(5 * time.Second)
	res, e := WgetFailover(url, selector, proxy, proxyIndexUsed, attempts)
	if e == nil {
		return res, e
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if len(proxy) != 0 {
		proxyURL := proxy[proxyIndexUsed]
		ctx, cancel = NewChromedpContext(90*time.Second,
			chromedp.ProxyServer(proxyURL),
		)
		slog.Info("proxy set for default chromeDP",
			slog.String("proxy url", proxy[proxyIndexUsed]),
		)
	} else {
		slog.Info("proxy set to nil for chromeDP")
		ctx, cancel = NewChromedpContext(90 * time.Second)
	}

	var screenShot []byte
	var HTMLContent string
	var err error
	if strings.Contains(url, "amazon") {
		err = chromedp.Run(ctx,
			StealthActions(url),
			chromedp.Navigate(url),
			chromedp.Sleep(time.Duration(rand.IntN(5)+2)*time.Second),
			chromedp.FullScreenshot(&screenShot, 70),
			chromedp.Evaluate(`document.querySelector('button.a-button-text[alt="Continue shopping"]')?.click()`, nil),
			chromedp.Sleep(15*time.Second),
			chromedp.OuterHTML("body", &HTMLContent),
		)
	} else {
		err = chromedp.Run(ctx,
			StealthActions(url),
			chromedp.Navigate(url),
			chromedp.Sleep(time.Duration(rand.IntN(10)+30)*time.Second),
			chromedp.FullScreenshot(&screenShot, 70),
			chromedp.OuterHTML("body", &HTMLContent),
		)
	}
	cancel()
	if err != nil {
		if len(proxy) != 0 {
			err2 := os.WriteFile("logs/proxyFailoverSS.png", screenShot, 0o644)
			err3 := os.WriteFile("logs/proxyFailoverHTML.html", []byte(HTMLContent), 0o644)
			time.Sleep(5 * time.Second)
			attempts = append(attempts, makeAttemptObject(types.CrawlerDefault,
				proxy[proxyIndexUsed], types.MethodChromeDP,
				errOrMsg(err, "Price Text is empty in chromeDP")))
			proxy = append(proxy[:proxyIndexUsed], proxy[proxyIndexUsed+1:]...)
			slog.Warn("ChromDP proxy failed, triggering default crawler without this proxy",
				slog.Any("error", err),
				slog.Any("write err1", err2),
				slog.Any("write err 2", err3),
				slog.Any("new Proxy Arr", proxy),
			)
			return GetPrice(url, selector, proxy, attempts)
		} else {
			slog.Error("no proxy ChromeDB also failed")
			err2 := os.WriteFile("logs/failoverSS.png", screenShot, 0o644)
			err3 := os.WriteFile("logs/failoverHTML.html", []byte(HTMLContent), 0o644)
			slog.Error("error in default chromedp", slog.String("selector", selector),
				slog.String("URL", url), slog.Any("ChromeDP Error", err),
				slog.Any("ScreenShot Write Error", err2), slog.Any("HTML Write Error", err3))
			attempts = append(attempts, makeAttemptObject(types.CrawlerDefault,
				types.ProxyDisabled, types.MethodChromeDP,
				errOrMsg(err, "Price Text is empty in chromeDP")))
			loggIncident(url, attempts, false)
			return 0, fmt.Errorf("selector %s not found for url %s, %w in default crawler", selector, url, err)
		}
	}

	res, err = ParseDefaultChromedpHTML(HTMLContent, selector)
	if err != nil {
		slog.Info("ChromeDP found Selector", slog.Int("Found Price", res))
		loggIncident(url, attempts, true)
	} else {
		slog.Error("ChromeDP failed")
		loggIncident(url, attempts, false)

	}

	return int(float64(res) * TaxRate), nil
}

func WgetFailover(url string, selector string, proxy []string, proxyIndexUsed int, attempts []*types.Attempt) (int, error) {
	slog.Warn("WgetFailover triggered",
		slog.String("URL", url),
		slog.String("Selector", selector))

	args := []string{
		"--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"--header=Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"--header=Accept-Language: en-US,en;q=0.5",
		"--header=Accept-Encoding: gzip, deflate",
		"--header=Connection: keep-alive",
		"--header=Upgrade-Insecure-Requests: 1",
		"--compression=auto",
		"-qO-",
		url,
	}
	if len(proxy) != 0 {
		args = append(args, "-e", "use_proxy=yes", "-e", "http_proxy="+proxy[proxyIndexUsed])
	}
	cmd := exec.Command("wget", args...)
	html, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("wget failed", slog.Any("error", err), slog.String("output", string(html)))
		return 0, fmt.Errorf("wget failed: %w, output: %s", err, string(html))
	}
	res, err := ParseDefaultChromedpHTML(string(html), selector)
	if err != nil {
		slog.Error("ParseDefaultChromedpHTML failed", slog.Any("error", err))
		return 0, err
	}

	slog.Info("WgetFailover found price", slog.Int("price", res))
	return int(float64(res) * TaxRate), nil
}
