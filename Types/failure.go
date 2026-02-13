package types

import "time"

type CrawlerType string

const (
	CrawlerEbay     CrawlerType = "ebay"
	CrawlerFacebook CrawlerType = "facebook"
	CrawlerDepop    CrawlerType = "depop"
	CrawlerDefault  CrawlerType = "default"
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
	Crawler   CrawlerType `bson:"Crawler"`
	Proxy     string      `bson:"Proxy"`
	Method    string      `bson:"Method"`
	Timestamp time.Time   `bson:"Timestamp"`
	Error     string      `bson:"Error"`
}

type Incident struct {
	StartTime time.Time  `bson:"StartTime"`
	URL       string     `bson:"URL"`
	Domain    string     `bson:"Domain"`
	Attempts  []*Attempt `bson:"Attempts"`
	Resolved  bool       `bson:"Resolved"`
}
