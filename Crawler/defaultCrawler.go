package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"time"

	logger "priceTracker/Logger"
	Proxy "priceTracker/Proxy"
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
func GetPrice(name, uri, querySelector string, proxy []string, attempts []*types.Attempt) (int, error) {
	var err, priceErr error
	var proxyIndex int
	var collyHTML string
	var c *colly.Collector
	res := 0
	crawled := false
	slog.Info("logging url", slog.String("URI", uri))
	if attempts == nil {
		attempts = []*types.Attempt{}
	}
	c, proxyIndex = initCrawler(uri, &proxy)
	c.OnHTML("body", func(h *colly.HTMLElement) {
		collyHTML, _ = h.DOM.Html()
	})
	DefaultParserCallback(c, querySelector, &crawled, &res, &priceErr)
	err = c.Visit(uri)
	c.Wait()
	proxyString := types.ProxyDisabled
	if len(proxy) != 0 {
		proxyString = proxy[proxyIndex]
	}
	logger.LogFileChannel <- makeCrawlFilesObject(name, types.CrawlerDefault, collyHTML, proxyString, nil)
	if !crawled {
		err = types.MakeError(types.ErrDefault, "element not found", uri)
	}
	if err != nil || priceErr != nil {
		slog.Warn("error in getting price in crawler, triggering chrome",
			slog.Any("Error", err), slog.Any("PriceErr", priceErr),
			slog.String("proxy", proxyString),
		)
		attempts = append(attempts, makeAttemptObject(types.CrawlerDefault, proxyString,
			types.MethodColly,
			firstErrorMsg(err, priceErr, "unknown error")))
		res, err2 := ChromeDPFailover(name, uri, querySelector, proxy, proxyIndex, attempts)
		return res, err2
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
		slog.Info("HTML Parser being called")
		*crawled = true
		*price, *priceErr = formatPrice(h.Text)
		c.OnHTMLDetach(querySelector)
	})
}

// ParseChromedpHTML - new function to parse chromedp output with colly
func ParseDefaultChromedpHTML(html string,
	querySelector, url string,
) (int, error) {
	// Create test server with the chromedp HTML
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer ts.Close()
	c, _ := initCrawler("", &[]string{})
	crawled := false
	price := 0
	var priceErr error

	DefaultParserCallback(c, querySelector, &crawled, &price, &priceErr)
	err := c.Visit(ts.URL)
	c.Wait()
	if !crawled && err == nil {
		slog.Error("Error was nil, but website not visited")
		err = types.MakeError(types.ErrDefault, "selector not found", url)
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
func ChromeDPFailover(name, url, selector string, proxy []string, proxyIndexUsed int, attempts []*types.Attempt) (int, error) {
	slog.Warn("ChromeDP and wget Triggered for default crawler",
		slog.String("URL", url), slog.String("Selector", selector),
	)
	time.Sleep(5 * time.Second)
	res, e := WgetFailover(url, selector, proxy, proxyIndexUsed, attempts)
	if e == nil {
		return res, e
	}
	time.Sleep(5 * time.Second)
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = NewChromedpContext(90, &proxy, proxyIndexUsed)

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
	proxyString := types.ProxyDisabled
	if len(proxy) != 0 {
		proxyString = proxy[proxyIndexUsed]
	}
	CrawlObj := makeCrawlFilesObject(name, types.CrawlerDefault,
		HTMLContent, proxyString, screenShot)
	if err != nil {
		logger.LogFileChannel <- CrawlObj
		attempts = append(attempts, makeAttemptObject(types.CrawlerDefault,
			proxyString, types.MethodChromeDP,
			errOrMsg(err, "Price Text is empty in chromeDP")))
		if len(proxy) != 0 {
			proxy = append(proxy[:proxyIndexUsed], proxy[proxyIndexUsed+1:]...)
			slog.Warn("ChromDP proxy failed, triggering default crawler without this proxy",
				slog.Any("error", err),
				slog.Any("new Proxy Arr", proxy),
			)
			return GetPrice(name, url, selector, proxy, attempts)
		} else {
			slog.Error("no proxy ChromeDB also failed")
			slog.Error("error in default chromedp", slog.String("selector", selector),
				slog.String("URL", url), slog.Any("ChromeDP Error", err),
			)
			loggIncident(url, attempts, false)
			logger.WriteLogFiles(CrawlObj)
			err = types.MakeError(types.ErrDefault, err.Error(), url)
			return 0, err
		}
	}

	res, err = ParseDefaultChromedpHTML(HTMLContent, selector, url)
	if err == nil {
		slog.Info("ChromeDP found Selector", slog.Int("Found Price", res))
		loggIncident(url, attempts, true)
		if len(proxy) != 0 {
			slog.Info("recording proxy success in chromdp")
			Proxy.ProxySuccessChannel <- proxy[proxyIndexUsed]
		}
	} else {
		slog.Error("ChromeDP failed")
		loggIncident(url, attempts, false)
		logger.WriteLogFiles(CrawlObj)
	}

	return int(float64(res) * TaxRate), err
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
	res, err := ParseDefaultChromedpHTML(string(html), selector, url)
	if err != nil {
		slog.Error("ParseDefaultChromedpHTML failed", slog.Any("error", err))
		return 0, err
	}

	slog.Info("WgetFailover found price", slog.Int("price", res))
	return int(float64(res) * TaxRate), nil
}
