// Package statistics provides failure tracking and incident logging for crawlers.
package types

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
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
var (
	IncidentChannel chan Incident
	IncidentCounter int
)

// SaveAttemptFunc is the function signature for saving attempts to the database.
type SaveAttemptFunc func(*Incident)

// StartIncidentListener starts a goroutine that listens for attempts on the channel
// and calls the provided database function to save them.
// This should be called once after InitAttemptChannel.
// The listener will exit when the done channel is closed.
func StartIncidentListener(dbFunc SaveAttemptFunc, done <-chan struct{}) {
	IncidentChannel = make(chan Incident, 100)
	IncidentCounter = 0
	go func() {
		for {
			select {
			case Incident := <-IncidentChannel:
				slog.Warn("logging Incident", slog.Any("incident", Incident))
				dbFunc(&Incident)
				IncidentCounter += 1
				if IncidentCounter >= 5 {
					slog.Info("Incidents Exceeding Limit, restarting vpn container")
					if err := RestartGluetun(); err != nil {
						slog.Error("failed to restart gluetun", slog.Any("error", err))
					}
					IncidentCounter = 0
				}
			case <-done:
				return
			}
		}
	}()
}

func RestartGluetun() error {
	client := &http.Client{Timeout: 10 * time.Second}

	for i := 0; i < 3; i++ {
		resp, err := client.Get("http://gluetun:8000/v1/vpn/status")
		if err != nil {
			slog.Error("failed to get VPN status", slog.Any("error", err))
			time.Sleep(2 * time.Second)
			continue
		}

		var status struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			slog.Error("failed to decode VPN status", slog.Any("error", err))
			resp.Body.Close()
			time.Sleep(2 * time.Second)
			continue
		}
		resp.Body.Close()

		if status.Status != "stopped" {
			slog.Info("VPN is running, cycling: stop then start")

			req, _ := http.NewRequest("PUT", "http://gluetun:8000/v1/vpn/status",
				strings.NewReader(`{"status":"stopped"}`))
			req.Header.Set("Content-Type", "application/json")
			client.Do(req)

			time.Sleep(2 * time.Second)

			req, _ = http.NewRequest("PUT", "http://gluetun:8000/v1/vpn/status",
				strings.NewReader(`{"status":"running"}`))
			req.Header.Set("Content-Type", "application/json")
			_, err = client.Do(req)
			if err != nil {
				slog.Error("failed to restart VPN", slog.Any("error", err))
				time.Sleep(2 * time.Second)
				continue
			}
			slog.Warn("Gluetun VPN restarted due to incident threshold")
			return nil
		}

		slog.Info("VPN already stopped, attempting to start")
		req, _ := http.NewRequest("PUT", "http://gluetun:8000/v1/vpn/status",
			strings.NewReader(`{"status":"running"}`))
		req.Header.Set("Content-Type", "application/json")
		_, err = client.Do(req)
		if err != nil {
			slog.Error("failed to start VPN", slog.Any("error", err))
			time.Sleep(2 * time.Second)
			continue
		}
		return nil
	}

	return errors.New("failed to restart VPN after 3 attempts")
}
