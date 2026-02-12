// Package statistics provides failure tracking and incident logging for crawlers.
package types

import (
	"log/slog"
	"time"
)

// CrawlerType identifies which crawler experienced the failure.
type CrawlerType string

const (
	CrawlerEbay     CrawlerType = "ebay"
	CrawlerFacebook CrawlerType = "facebook"
	CrawlerDepop    CrawlerType = "depop"
	CrawlerDefault  CrawlerType = "default"
)

// ProxyType identifies the proxy configuration.
type ProxyType string

const (
	ProxyEnabled  ProxyType = "proxy"
	ProxyDisabled ProxyType = "no_proxy"
)

// MethodType identifies the scraping method used.
type MethodType string

const (
	MethodColly    MethodType = "colly"
	MethodChromeDP MethodType = "chromedp"
)

// Attempt represents a single attempt to scrape data.
type Attempt struct {
	Crawler   CrawlerType `bson:"Crawler"`   // Which crawler was used
	Proxy     ProxyType   `bson:"Proxy"`     // Proxy configuration
	Method    MethodType  `bson:"Method"`    // Scraping method
	Timestamp time.Time   `bson:"Timestamp"` // When the attempt was made
	Error     string      `bson:"Error"`     // Error message (empty if success)
}

// Incident represents a complete failure event with all attempts.
type Incident struct {
	StartTime time.Time  `bson:"StartTime"` // When the incident started
	URL       string     `bson:"URL"`       // URL being crawled
	Domain    string     `bson:"Domain"`
	Attempts  []*Attempt `bson:"Attempts"` // All attempts made
	Resolved  bool       `bson:"Resolved"` // Whether it was eventually resolved
}

// IncidentChannel is the channel for sending attempts to be persisted.
// Buffered to prevent blocking crawlers.
var IncidentChannel chan Incident

// SaveAttemptFunc is the function signature for saving attempts to the database.
type SaveAttemptFunc func(*Incident)

// StartAttemptListener starts a goroutine that listens for attempts on the channel
// and calls the provided database function to save them.
// This should be called once after InitAttemptChannel.
// The listener will exit when the done channel is closed.
func StartAttemptListener(dbFunc SaveAttemptFunc, done <-chan struct{}) {
	IncidentChannel = make(chan Incident, 100)
	go func() {
		for {
			select {
			case Incident := <-IncidentChannel:
				slog.Warn("logging Incident", slog.Any("incident", Incident))
				dbFunc(&Incident)
			case <-done:
				return
			}
		}
	}()
}
