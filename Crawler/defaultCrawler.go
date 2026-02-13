package crawler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"time"

	"priceTracker/Logger"

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
func GetPrice(uri string, querySelector string, proxy bool, attempts []*logger.Attempt) (int, error) {
	var err, priceErr error
	res := 0
	crawled := false
	slog.Info("logging url", slog.String("URI", uri), slog.Bool("proxy", proxy))
	c := initCrawler()
	if attempts == nil {
		attempts = []*logger.Attempt{}
	}
	if !proxy {
		c.SetProxyFunc(nil)
	}
	c.OnHTML(querySelector, func(h *colly.HTMLElement) {
		crawled = true
		res, priceErr = formatPrice(h.Text)
		c.OnHTMLDetach(querySelector)
	})
	var collyHTML string
	c.OnHTML("body", func(h *colly.HTMLElement) {
		collyHTML, _ = h.DOM.Html()
	})
	err = c.Visit(uri)

	c.Wait()
	if !crawled {
		err = errors.New("Error in default crawler: could not crawl, html element does not exist")
	}
	if err != nil || priceErr != nil {
		var res int
		var err2 error
		os.WriteFile("logs/collyHTML.html", []byte(collyHTML), 0o644)
		if proxy {
			slog.Warn("error in getting price in crawler, triggering proxy chrome",
				slog.Any("Error", err), slog.Any("PriceErr", priceErr))
			attempts = append(attempts, &logger.Attempt{
				Crawler:   logger.CrawlerDefault,
				Proxy:     logger.ProxyEnabled,
				Method:    logger.MethodColly,
				Timestamp: time.Now(),
				Error: func(err error) string {
					if err != nil {
						return err.Error()
					} else {
						return priceErr.Error()
					}
				}(err),
			})
			res, err2 = ChromeDPFailover(uri, querySelector, true, attempts)
			return res, err2
		} else {
			slog.Warn("no proxy default crawler failed, triggering chromeDPFailover no proxy",
				slog.Any("Error", err2), slog.Int("Price", res))
			attempts = append(attempts, &logger.Attempt{
				Crawler:   logger.CrawlerDefault,
				Proxy:     logger.ProxyDisabled,
				Method:    logger.MethodColly,
				Timestamp: time.Now(),
				Error: func(err error) string {
					if err != nil {
						return err.Error()
					} else {
						return priceErr.Error()
					}
				}(err),
			})
			res, err2 = ChromeDPFailover(uri, querySelector, false, attempts)
			return res, err2
		}
	}
	if len(attempts) != 0 {
		logger.IncidentChannel <- logger.Incident{
			StartTime: time.Now(),
			URL:       uri,
			Domain:    ExtractDomainName(uri),
			Attempts:  attempts,
			Resolved:  true,
		}
	}
	return int(float64(res) * TaxRate), err
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
func ChromeDPFailover(url string, selector string, proxy bool, attempts []*logger.Attempt) (int, error) {
	slog.Warn("ChromDP Triggered for default crawler",
		slog.String("URL", url), slog.String("Selector", selector),
		slog.Bool("Proxy", proxy),
	)
	var ctx context.Context
	var cancel context.CancelFunc
	if proxy {
		ctx, cancel = NewChromedpContext(90*time.Second, chromedp.ProxyServer("http://gluetun:8888"))
	} else {
		ctx, cancel = NewChromedpContext(90 * time.Second)
	}

	var priceText string
	var screenShot []byte
	var HTMLContent string
	var err error
	js := fmt.Sprintf(`document.querySelector("%s")?.innerText || ""`, selector)
	if strings.Contains(url, "amazon") {
		err = chromedp.Run(ctx,
			chromedp.Navigate(url),
			StealthActions(),
			chromedp.Sleep(time.Duration(rand.IntN(10)+15)*time.Second),
			chromedp.FullScreenshot(&screenShot, 70),
			chromedp.OuterHTML("body", &HTMLContent),
			chromedp.Evaluate(`document.querySelector('button.a-button-text[alt="Continue shopping"]')?.click()`, nil),
			chromedp.Sleep(5*time.Second),
			// chromedp.Text(selector, &priceText, chromedp.ByQuery),
			chromedp.Evaluate(js, &priceText),
		)
	} else {
		err = chromedp.Run(ctx,
			chromedp.Navigate(url),
			StealthActions(),
			chromedp.Sleep(time.Duration(rand.IntN(10)+30)*time.Second),
			chromedp.FullScreenshot(&screenShot, 70),
			chromedp.OuterHTML("body", &HTMLContent),
			chromedp.Evaluate(js, &priceText),
		)
	}
	cancel()
	if priceText == "" {
		if proxy {
			err2 := os.WriteFile("logs/proxyFailoverSS.png", screenShot, 0o644)
			err3 := os.WriteFile("logs/proxyFailoverHTML.html", []byte(HTMLContent), 0o644)
			time.Sleep(5 * time.Second)
			slog.Warn("ChromDP proxy failed, triggering no proxy default crawler",
				slog.Any("error", err),
				slog.Any("priceText", priceText),
				slog.Any("write err1", err2),
				slog.Any("write err 2", err3))
			attempts = append(attempts, &logger.Attempt{
				Crawler:   logger.CrawlerDefault,
				Proxy:     logger.ProxyEnabled,
				Method:    logger.MethodChromeDP,
				Timestamp: time.Now(),
				Error: func(err error) string {
					if err != nil {
						return err.Error()
					} else {
						return errors.New("Price Text is empty in chromeDP").Error()
					}
				}(err),
			})
			return GetPrice(url, selector, false, attempts)
		} else {
			slog.Error("no proxy ChromeDB also failed")
			err2 := os.WriteFile("logs/failoverSS.png", screenShot, 0o644)
			err3 := os.WriteFile("logs/failoverHTML.html", []byte(HTMLContent), 0o644)
			slog.Error("error in default chromedp", slog.String("selector", selector),
				slog.String("URL", url), slog.Any("ChromeDP Error", err),
				slog.Any("ScreenShot Write Error", err2), slog.Any("HTML Write Error", err3))
			attempts = append(attempts, &logger.Attempt{
				Crawler:   logger.CrawlerDefault,
				Proxy:     logger.ProxyDisabled,
				Method:    logger.MethodChromeDP,
				Timestamp: time.Now(),
				Error: func(err error) string {
					if err != nil {
						return err.Error()
					} else {
						return errors.New("Price Text is empty in chromeDP").Error()
					}
				}(err),
			})
			logger.IncidentChannel <- logger.Incident{
				StartTime: time.Now(),
				Domain:    ExtractDomainName(url),
				URL:       url,
				Attempts:  attempts,
				Resolved:  false,
			}
			return 0, fmt.Errorf("selector %s not found for url %s, %w in default crawler", selector, url, err)
		}
	}

	slog.Info("ChromeDP found Selector", slog.String("Found HTML Element", priceText))
	logger.IncidentChannel <- logger.Incident{
		StartTime: time.Now(),
		URL:       ExtractDomainName(url),
		Attempts:  attempts,
		Resolved:  true,
	}
	// Parse price
	price, err := formatPrice(priceText)
	if err != nil || price == 0 {
		os.WriteFile("logs/failoverHTML.html", []byte(HTMLContent), 0o644)
		os.WriteFile("logs/failoverSS.png", screenShot, 0o644)
		return 0, fmt.Errorf("failed to parse price '%s': %w in default crawler", priceText, err)
	}

	return int(float64(price) * TaxRate), nil
}
