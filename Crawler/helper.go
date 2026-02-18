package crawler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

// TaxRate is applied to prices to account for taxes (10% by default).
var TaxRate = 1.1

// initCrawler creates a colly collector with rate limiting and headers configured
// to avoid detection. Uses a proxy by default.
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
	return c
}

// NewChromedpContext creates a chromedp context with anti-detection settings configured.
// It sets up a headless browser with stealth options to avoid bot detection.
//
// Parameters:
//   - timeout: the timeout for the context
//   - extraOpts: optional additional chromedp options
//
// Returns the context and cancel function.
func NewChromedpContext(timeout time.Duration, extraOpts ...chromedp.ExecAllocatorOption) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("log-level", "3"),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
	)

	opts = append(opts, extraOpts...)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	ctx, timeoutCancel := context.WithTimeout(ctx, timeout)

	cancel := func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}

	return ctx, cancel
}

// StealthActions returns chromedp actions that help evade bot detection.
// It masks the headless browser by setting typical browser properties.
//
// Returns a chromedp action that executes stealth JavaScript.
func StealthActions() chromedp.Action {
	return chromedp.Evaluate(`
		// Webdriver
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined
		});
		
		// Plugins
		Object.defineProperty(navigator, 'plugins', {
			get: () => [
				{name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer'},
				{name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai'},
				{name: 'Native Client', filename: 'internal-nacl-plugin'}
			]
		});
		
		// Languages
		Object.defineProperty(navigator, 'languages', {
			get: () => ['en-US', 'en']
		});
		
		// Chrome runtime
		window.chrome = {
			runtime: {
				connect: () => {},
				sendMessage: () => {}
			}
		};
		
		// Permissions
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' ?
				Promise.resolve({ state: Notification.permission }) :
				originalQuery(parameters)
		);
		
		// Hardware
		Object.defineProperty(navigator, 'hardwareConcurrency', {
			get: () => 8
		});
	`, nil)
}

// GetOpenGraphPic retrieves the product image URL from a webpage.
// Supports Amazon, Best Buy, and generic Open Graph image tags.
//
// Parameters:
//   - url: the URL to extract the image from
//
// Returns the image URL or an empty string if not found.
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
	} else {
		c.OnHTML("meta[property='og:image']", func(e *colly.HTMLElement) {
			imgURL = e.Attr("content")
			visited = true
		})
	}
	err := c.Visit(url)
	if err != nil || !visited {
		slog.Warn("could not get Open Graph picture", slog.Any("ERROR: ", err), slog.Any("Visited: ", visited))

		// Fallback to chromedp for Amazon
		if strings.Contains(url, "amazon") {
			imgURL = getAmazonImageChromedp(url)
		}

		if imgURL == "" {
			return ""
		}
	}
	c.Wait()
	return imgURL
}

func getAmazonImageChromedp(url string) string {
	var ctx context.Context
	var cancel context.CancelFunc
	ctx, cancel = NewChromedpContext(90 * time.Second)
	defer cancel()

	var imgURL string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		StealthActions(),
		chromedp.Sleep(10*time.Second),
		chromedp.Evaluate(`document.querySelector('button.a-button-text[alt="Continue shopping"]')?.click()`, nil),
		chromedp.Sleep(2*time.Second),
		chromedp.Evaluate(`document.querySelector('img#landingImage')?.src || ""`, &imgURL),
	)
	if err != nil || imgURL == "" {
		slog.Error("chromedp failed to get Amazon image", slog.Any("error", err))
		return ""
	}

	slog.Info("Image URL found")
	return imgURL
}

func ExtractDomainName(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	// Remove www.
	url = strings.TrimPrefix(url, "www.")
	// Split by . and get first part
	parts := strings.Split(url, ".")
	return parts[0]
}

func formatPrice(priceStr string) (int, error) {
	ret := strings.ReplaceAll(priceStr, "$", "")
	ret = strings.ReplaceAll(ret, "\n", "")
	ret = strings.ReplaceAll(ret, ",", "")
	ret = strings.TrimSpace(ret)
	ret = strings.Split(ret, ".")[0]
	res, err := strconv.Atoi(ret)
	return res, err
}
