package crawler

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	logger "priceTracker/Logger"
	types "priceTracker/Types"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/dlclark/regexp2"
	"github.com/enetx/g"
	"github.com/enetx/surf"
	"github.com/gocolly/colly/v2"
)

// TaxRate is applied to prices to account for taxes (10% by default).
var (
	TaxRate        = 1.1
	excludeRegexes []*regexp2.Regexp
	HttpClients    []*func() *surf.Builder

	// Step 2: reused HTTP clients per proxy exit so cookies/TLS
	// fingerprint stay consistent instead of stateless per request.
	clientCacheMu sync.Mutex
	clientCache   = make(map[string]*http.Client)
	warmedMu      sync.Mutex
	warmedClients = make(map[string]bool)
)

// initCrawler creates a colly collector with rate limiting and headers configured
// to avoid detection. Uses a proxy by default.
//
// Step 2 notes:
//   - HTTP client (TLS fingerprint + cookies) is reused per proxy exit.
//   - No manual Host header (net/http sets it from URL).
//   - Entry navigation uses Sec-Fetch-Site: none + empty Referer.
//   - 403/429 is surfaced explicitly via OnError logging.
func initCrawler(url string, proxy *[]string) (*colly.Collector, int) {
	// --------------------------- initiaize scrapper headers and settings ------- //
	var c *colly.Collector
	c = colly.NewCollector(
		colly.MaxDepth(1),
		colly.AllowURLRevisit(),
	)
	c.SetRequestTimeout(60 * time.Second)
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*ebay.*",
		Delay:       1 * time.Minute,
		RandomDelay: 3 * time.Minute,
	})
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       2 * time.Second,
		RandomDelay: 1 * time.Second,
	})

	_, Referer := FormatHostAndRefererUrls(url)
	isEntryNavigation := true
	c.OnRequest(func(r *colly.Request) {
		// Do NOT overwrite User-Agent: surf impersonation sets a
		// consistent UA matching the TLS fingerprint.
		r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
		r.Headers.Set("Cache-Control", "max-age=0")
		r.Headers.Set("DNT", "1")
		r.Headers.Set("Upgrade-Insecure-Requests", "1")
		r.Headers.Set("Sec-Fetch-Dest", "document")
		r.Headers.Set("Sec-Fetch-Mode", "navigate")
		r.Headers.Set("Sec-Fetch-User", "?1")
		if isEntryNavigation {
			// First hit to eBay: no Referer, site=none.
			// same-origin here is a strong bot signal.
			r.Headers.Set("Sec-Fetch-Site", "none")
		} else {
			r.Headers.Set("Referer", Referer)
			r.Headers.Set("Sec-Fetch-Site", "same-origin")
		}
		// NOTE: no manual Host header, no manual Accept-Encoding
		// (transport negotiates gzip/br/zstd itself).
	})
	var proxyIndex int
	var proxyURL string
	if len(*proxy) > 0 {
		proxyIndex = rand.IntN(len(*proxy))
		proxyURL = (*proxy)[proxyIndex]
		slog.Info("proxy set for crawler",
			slog.Any("proxyArr", proxy),
			slog.String("proxy url", (*proxy)[proxyIndex]),
		)
	} else {
		proxyURL = ""
		slog.Info("no proxy set, running on home IP")
	}
	httpClient := getCachedClient(proxyURL)
	c.SetClient(httpClient)
	c.OnResponse(func(r *colly.Response) {
		slog.Info("Response received",
			slog.Int("status", r.StatusCode),
			slog.String("url", r.Request.URL.String()),
		)
	})
	c.OnError(func(r *colly.Response, err error) {
		status := 0
		bodySnippet := ""
		reqURL := ""
		if r != nil {
			status = r.StatusCode
			reqURL = r.Request.URL.String()
			if len(r.Body) > 0 {
				n := min(len(r.Body), 500)
				bodySnippet = string(r.Body[:n])
			}
		}
		slog.Error("Colly request failed",
			slog.Any("error", err),
			slog.Int("status", status),
			slog.String("url", reqURL),
			slog.String("body_snippet", bodySnippet),
		)
		if status == 403 || status == 429 {
			slog.Warn("eBay block detected (403/429): rotate proxy, backoff, do not retry same IP immediately",
				slog.Int("status", status),
				slog.String("url", reqURL),
			)
		}
	})
	return c, proxyIndex
}

// getCachedClient returns a reused *http.Client per proxy exit so cookies
// and the impersonated TLS fingerprint stay stable. The underlying surf
// builder is still chosen randomly once per proxyURL, then cached.
func getCachedClient(proxyURL string) *http.Client {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	if c, ok := clientCache[proxyURL]; ok {
		return c
	}
	c := generateRandomClient(proxyURL)
	if c.Jar == nil {
		jar, _ := cookiejar.New(nil)
		c.Jar = jar
	}
	clientCache[proxyURL] = c
	go warmupEbaySession(c, proxyURL)
	return c
}

// warmupEbaySession seeds cookies with a single homepage GET per proxy exit.
func warmupEbaySession(c *http.Client, proxyURL string) {
	warmedMu.Lock()
	if warmedClients[proxyURL] {
		warmedMu.Unlock()
		return
	}
	warmedClients[proxyURL] = true
	warmedMu.Unlock()

	req, err := http.NewRequest("GET", "https://www.ebay.com/", nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	resp, err := c.Do(req)
	if err != nil {
		slog.Warn("ebay warmup failed", slog.Any("error", err), slog.String("proxy", proxyURL))
		return
	}
	defer resp.Body.Close()
	slog.Info("ebay warmup done", slog.Int("status", resp.StatusCode), slog.String("proxy", proxyURL))
}

func InitAntiTLSClients() {
	surfClient := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			MacOS(). // Randomly selects Windows, macOS, Linux, Android, or iOS
			Chrome() // Latest Firefox v147
	}
	surfClient2 := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			Windows(). // Randomly selects Windows, macOS, Linux, Android, or iOS
			Firefox()  // Latest Firefox v147
	}
	surfClient3 := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			Android(). // Randomly selects Windows, macOS, Linux, Android, or iOS
			Chrome()
	}
	surfClient4 := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			Windows(). // Randomly selects Windows, macOS, Linux, Android, or iOS
			Chrome()
	}
	surfClient5 := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			MacOS(). // Randomly selects Windows, macOS, Linux, Android, or iOS
			Firefox()
	}
	surfClient6 := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			IOS(). // Randomly selects Windows, macOS, Linux, Android, or iOS
			Chrome()
	}
	surfClient7 := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			IOS(). // Randomly selects Windows, macOS, Linux, Android, or iOS
			Firefox()
	}
	surfClient8 := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			Android(). // Randomly selects Windows, macOS, Linux, Android, or iOS
			Firefox()
	}
	surfClient9 := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			Linux().
			Chrome()
	}
	surfClient10 := func() *surf.Builder {
		return surf.NewClient().
			Builder().
			Impersonate().
			Linux().
			Firefox()
	}
	HttpClients = append(HttpClients, &surfClient, &surfClient2, &surfClient3,
		&surfClient4, &surfClient5, &surfClient6, &surfClient7, &surfClient8,
		&surfClient9, &surfClient10)
}

func generateRandomClient(proxyURL string) *http.Client {
	index := rand.IntN(len(HttpClients))
	ClientFunc := *HttpClients[index]
	slog.Info("picking surf client", slog.Int("Client", index))
	return ClientFunc().Proxy(g.String(proxyURL)).Build().Unwrap().Std()
}

// NewChromedpContext creates a chromedp context with anti-detection settings configured.
// It sets up a headless browser with stealth options to avoid bot detection.
//
// Step 7: realistic viewport/lang/timezone, persistent stealth via
// AddScriptToEvaluateOnNewDocument (see StealthInitScript). Callers must
// Navigate AFTER StealthInitScript so patches survive page loads.
//
// Parameters:
//   - timeout: the timeout for the context
//   - extraOpts: optional additional chromedp options
//
// Returns the context and cancel function.
func NewChromedpContext(timeout time.Duration, proxy *[]string, proxyIndex int) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath("/usr/bin/chromium"),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("exclude-switches", "enable-automation"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-software-rasterizer", true),
		chromedp.Flag("log-level", "3"),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
		chromedp.Flag("headless", "new"),
		// Step 7: real desktop viewport + locale so Akamai sees a
		// consistent fingerprint instead of default 800x600 headless.
		chromedp.Flag("window-size", "1366,768"),
		chromedp.Flag("lang", "en-US"),
		chromedp.Flag("accept-lang", "en-US,en;q=0.9"),
		chromedp.Flag("timezone", "America/Los_Angeles"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)

	var proxyURL string
	if len(*proxy) != 0 {
		proxyURL = (*proxy)[proxyIndex]
		opts = append(opts, chromedp.ProxyServer(proxyURL))
		slog.Info("proxy set for crawler",
			slog.Any("proxyArr", proxy),
			slog.String("proxy url", (*proxy)[proxyIndex]),
		)
	} else {
		slog.Info("no proxy set, running on home IP")
	}
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

// stealthInitJS is injected via page.AddScriptToEvaluateOnNewDocument so it
// runs on every navigation, including the eBay search page itself.
// The old StealthActions ran chromedp.Evaluate on about:blank BEFORE Navigate,
// so all patches were lost on navigation.
const stealthInitJS = `
(() => {
  Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  Object.defineProperty(navigator, 'plugins', {
    get: () => [
      {name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer'},
      {name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai'},
      {name: 'Native Client', filename: 'internal-nacl-plugin'}
    ]
  });
  Object.defineProperty(navigator, 'languages', { get: () => ['en-US', 'en'] });
  Object.defineProperty(navigator, 'platform', { get: () => 'MacIntel' });
  Object.defineProperty(navigator, 'vendor', { get: () => 'Google Inc.' });
  Object.defineProperty(navigator, 'hardwareConcurrency', { get: () => 8 });
  Object.defineProperty(navigator, 'deviceMemory', { get: () => 8 });
  window.chrome = { runtime: { connect: () => {}, sendMessage: () => {} } };
  const originalQuery = window.navigator.permissions.query;
  window.navigator.permissions.query = (parameters) => (
    parameters.name === 'notifications' ?
      Promise.resolve({ state: Notification.permission }) :
      originalQuery(parameters)
  );
})();
`

// StealthInitScript persists stealth patches across navigations.
func StealthInitScript() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(stealthInitJS).Do(ctx)
		return err
	})
}

// StealthActions returns chromedp actions that help evade bot detection.
// It masks the headless browser by setting typical browser properties.
//
// Kept for backwards compat: now delegates to StealthInitScript so patches
// survive Navigate. Call as: StealthInit, Navigate, WaitVisible(selector).
// Returns a chromedp action that executes stealth JavaScript.
func StealthActions(url string) chromedp.Action {
	return chromedp.Tasks{
		StealthInitScript(),
		chromedp.EmulateViewport(1366, 768),
	}
}

// GetOpenGraphPic retrieves the product image URL from a webpage.
// Supports Amazon, Best Buy, and generic Open Graph image tags.
//
// Parameters:
//   - url: the URL to extract the image from
//
// Returns the image URL or an empty string if not found.
func GetOpenGraphPic(url string) string {
	c, _ := initCrawler(url, &[]string{})
	visited := false
	imgURL := ""
	if strings.Contains(url, "amazon") ||
		strings.Contains(url, "https://a.co") {
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
		if strings.Contains(url, "amazon") ||
			strings.Contains(url, "https://a.co") {
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
	ctx, cancel = NewChromedpContext(90*time.Second, &[]string{}, 0)
	defer cancel()

	var imgURL string
	err := chromedp.Run(ctx,
		StealthActions(url),
		chromedp.Navigate(url),
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

func FormatHostAndRefererUrls(url string) (string, string) {
	domain := ExtractDomainName(url)
	Host := "www." + domain + ".com"
	Referer := "https://" + Host
	return Host, Referer
}

func formatPrice(priceStr string) (int, error) {
	slog.Info("format price called", slog.String("input string", priceStr))
	ret := strings.ReplaceAll(priceStr, "$", "")
	ret = strings.ReplaceAll(ret, "\n", "")
	ret = strings.ReplaceAll(ret, ",", "")
	ret = strings.TrimSpace(ret)
	ret = strings.Split(ret, ".")[0]
	res, err := strconv.Atoi(ret)
	return res, err
}

// pre compile exception regex patterns
func init() {
	excludepatterns := []string{
		`\bfor parts`,
		`\bbroken`,
		`\baccessories\b`,
		`(?=.*\bonly\b)(?=.*\bbox\b)`,
		`\bempty box`,
		`\bcable\b`,
		`\bdongle\b`,
		`\bkids\b`,
		`\bjunior\b`,
		`read`,
		`\bstand\b`,
		`\badapter\b`,
		`\bdefective`,
		`damage`,
		`problem`,
		`replacement`,
		`bracket`,
		`water block`,
		`waterblock`,
		`\boem\b`,
		`board`,
		`\bdock\b`,
	}

	for _, pattern := range excludepatterns {
		re, err := regexp2.Compile(pattern, 0)
		if err != nil {
			continue
		}
		excludeRegexes = append(excludeRegexes, re)
	}
}

type ItemRegexInfo struct {
	IncludeRegex   [][]*regexp.Regexp
	IncludeSpecial [][]string
	ExcludeRegex   []*regexp.Regexp
	ExcludeSpecial []string
}

func QuoteNormalizer(listingTitle string) string {
	listingTitle = strings.ReplaceAll(listingTitle, "\u201C", `"`) // Left curly double quote "
	listingTitle = strings.ReplaceAll(listingTitle, "\u201D", `"`) // Right curly double quote "
	listingTitle = strings.ReplaceAll(listingTitle, `\"`, `"`)     // Escaped \"

	// Single quotes - curly quotes and escaped quotes
	listingTitle = strings.ReplaceAll(listingTitle, "\u2018", `'`) // Left curly single quote '
	listingTitle = strings.ReplaceAll(listingTitle, "\u2019", `'`) // Right curly single quote '
	listingTitle = strings.ReplaceAll(listingTitle, `\'`, `'`)     // Escaped \'

	listingTitle = strings.ToLower(listingTitle)
	return listingTitle
}

// initTitleRegex compiles regex patterns for item matching and exclusion.
// Words with special characters are matched exactly; others use word boundaries.
// Returns inclusion patterns, inclusion special words, exclusion patterns, and exclusion special words.
func InitTitleRegex(itemNames []string, exclusionQueries []string) *ItemRegexInfo {
	var (
		allRegexPatterns [][]*regexp.Regexp
		allSpecialWords  [][]string
	)
	slog.Info("initializing regex queries for",
		slog.Any("name arr", itemNames),
		slog.Any("exclusion queries", exclusionQueries))
	for _, itemName := range itemNames {
		words := strings.Fields(strings.ToLower(itemName))
		var regexPatterns []*regexp.Regexp
		var specialWords []string

		for _, word := range words {
			word = QuoteNormalizer(word)
			if strings.ContainsAny(word, "./-\"'()[]{}") {
				// Has special characters - add to string array
				specialWords = append(specialWords, word)
			} else {
				// Normal word - compile regex with word boundaries
				pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
				regexPatterns = append(regexPatterns, pattern)
			}
		}

		allRegexPatterns = append(allRegexPatterns, regexPatterns)
		allSpecialWords = append(allSpecialWords, specialWords)
	}

	// Process user-defined exclusion queries with word boundaries
	var exclusionRegexes []*regexp.Regexp
	var exclusionSpecialWords []string

	for _, query := range exclusionQueries {
		query = strings.ToLower(query)
		if strings.ContainsAny(query, "./-\"'()[]{}") {
			// Has special characters - add to string array for exact matching
			exclusionSpecialWords = append(exclusionSpecialWords, query)
		} else {
			// Normal word - compile regex with word boundaries
			pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(query) + `\b`)
			exclusionRegexes = append(exclusionRegexes, pattern)
		}
	}

	return &ItemRegexInfo{
		IncludeRegex:   allRegexPatterns,
		IncludeSpecial: allSpecialWords,
		ExcludeRegex:   exclusionRegexes,
		ExcludeSpecial: exclusionSpecialWords,
	}
}

// checks the title to make sure the name is in the title and
// no unwanted returned results by ebay
//
//
//
// Use word boundaries for normal words
// i needed to use boundries and cant just use simple
// strings.contains bc otherwise it includes 321up with 321upx
// it is a lot slower tho, but since its running longterm
// it doesnt really matter i think

// titleCorrectnessCheck validates if a listing title matches inclusion patterns
// and does not match exclusion patterns. It uses word boundaries for normal words
// to prevent partial matches (e.g., "32UP" matching "32UPX").
//
// Returns true if the title passes all checks (matches inclusion, no exclusions).
func titleCorrectnessCheck(listingTitle string, queries *ItemRegexInfo) bool {
	// Normalize all quote variations to standard keyboard quotes
	// Double quotes - curly quotes and escaped quotes
	listingTitle = QuoteNormalizer(listingTitle)
	atLeastOneMatched := false
outerloop:
	for i := range queries.IncludeRegex {
		// Check regex patterns for this itemName
		for _, pattern := range queries.IncludeRegex[i] {
			if !pattern.MatchString(listingTitle) {
				slog.Info("not matching keyword",
					slog.String("title", listingTitle),
					slog.String("pattern", pattern.String()),
				)
				continue outerloop
			}
		}

		// Check special character words for this itemName
		for _, word := range queries.IncludeSpecial[i] {
			if !strings.Contains(listingTitle, word) {
				slog.Info("not matching special char keyword",
					slog.String("title", listingTitle),
					slog.String("word missing", word),
				)
				continue outerloop
			}
		}

		atLeastOneMatched = true
		break
	}

	if !atLeastOneMatched {
		return false
	}

	// Check hardcoded global exclusions (using regexp2)
	for _, re := range excludeRegexes {
		match, err := re.MatchString(listingTitle)
		if err != nil {
			continue
		}
		if match {
			return false // Hardcoded exclusion matched
		}
	}

	// Check user-defined exclusion regexes
	for _, pattern := range queries.ExcludeRegex {
		if pattern.MatchString(listingTitle) {
			slog.Info("excluding title due to user-defined exclusion pattern",
				slog.String("title", listingTitle))
			return false
		}
	}

	// Check user-defined exclusion special words
	for _, word := range queries.ExcludeSpecial {
		if strings.Contains(listingTitle, word) {
			slog.Info("excluding title due to user-defined exclusion word",
				slog.String("title", listingTitle))
			return false
		}
	}

	return true // Title is valid
}

func errOrMsg(err error, defaultMsg string) string {
	if err != nil {
		return err.Error()
	}
	return defaultMsg
}

func firstErrorMsg(err1, err2 error, fallback string) string {
	if err1 != nil {
		return err1.Error()
	}
	if err2 != nil {
		return err2.Error()
	}
	return fallback
}

func makeAttemptObject(crawler, proxy, method, errorMsg string) *types.Attempt {
	return &types.Attempt{
		Crawler:   crawler,
		Proxy:     proxy,
		Method:    method,
		Timestamp: time.Now(),
		Error:     errorMsg,
	}
}

func loggIncident(url string, attempts []*types.Attempt, resolved bool) {
	logger.IncidentChannel <- types.Incident{
		StartTime: time.Now(),
		Domain:    ExtractDomainName(url),
		URL:       url,
		Attempts:  attempts,
		Resolved:  resolved,
	}
}

func makeCrawlFilesObject(Name, CrawlType, HTMLString, proxy string,
	ScreenShot []byte,
) *logger.CrawlFiles {
	return &logger.CrawlFiles{
		Proxy:      proxy,
		ItemName:   Name,
		CrawlType:  CrawlType,
		HTMLString: HTMLString,
		ScreenShot: ScreenShot,
	}
}
