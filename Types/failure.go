package types

import "time"

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
