package types

import (
	"errors"
	"time"
)

const (
	CrawlerEbay     string = "ebay"
	CrawlerFacebook string = "facebook"
	CrawlerDepop    string = "depop"
	CrawlerDefault  string = "default"
)

const (
	ProxyEnabled  string = "proxy"
	ProxyDisabled string = "no_proxy"
)

const (
	MethodColly    string = "colly"
	MethodChromeDP string = "chromedp"
)

type Attempt struct {
	Crawler   string    `bson:"Crawler"`
	Proxy     string    `bson:"Proxy"`
	Method    string    `bson:"Method"`
	Timestamp time.Time `bson:"Timestamp"`
	Error     string    `bson:"Error"`
}

type Incident struct {
	StartTime time.Time  `bson:"StartTime"`
	URL       string     `bson:"URL"`
	Domain    string     `bson:"Domain"`
	Attempts  []*Attempt `bson:"Attempts"`
	Resolved  bool       `bson:"Resolved"`
}

type CrawlError struct {
	Err error
	URL string
}

var (
	ErrEbay     = &CrawlError{Err: errors.New("error in crawling ebay")}
	ErrFacebook = &CrawlError{Err: errors.New("error in crawling facebook")}
	ErrDepop    = &CrawlError{Err: errors.New("error in crawling depop")}
	ErrDefault  = &CrawlError{Err: errors.New("error in crawling url")}
)

func (err *CrawlError) Error() string { return err.Err.Error() }
func WithURL(err error, url string) *CrawlError {
	return &CrawlError{Err: err, URL: url}
}
