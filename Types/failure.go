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
	MethodWGet     string = "wget"
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
	SentenialErr error
	Err          error
	URL          string
}

var (
	ErrEbay     error = errors.New("error in crawling ebay")
	ErrFacebook       = errors.New("error in crawling facebook")
	ErrDepop          = errors.New("error in crawling depop")
	ErrDefault        = errors.New("error in crawling url")
)

func (err CrawlError) Error() string {
	return errors.Join(err.SentenialErr, err.Err).Error() + err.URL
}

func MakeError(sentenialErr error, errString string, url string) *CrawlError {
	return &CrawlError{SentenialErr: sentenialErr, Err: errors.New(errString), URL: url}
}
