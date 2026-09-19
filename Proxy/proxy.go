package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	types "priceTracker/Types"
)

var (
	FailureRestartThreshold = 5
	SuccessRestartThreshold = 15
	// Step 5: minimum time between VPN restarts per proxy so we don't
	// rapidly cycle the same flagged exit IP.
	RestartCooldown       = 10 * time.Minute
	ProxyList             []string
	ProxyIncidentCounterMap map[string]int
	ProxyCounterChannel   chan []*types.Attempt
	ProxySuccessChannel   chan string
	ProxySuccessCounterMap map[string]int
	lastRestartMap        map[string]time.Time
)

func Init() {
	slog.Info("initializeing proxy rotator module")
	lastRestartMap = make(map[string]time.Time)
	if zip := strings.TrimSpace(os.Getenv("EBAY_ZIP")); zip != "" {
		slog.Info("EBAY_ZIP set, ensure VPN SERVER_CITIES matches it",
			slog.String("EBAY_ZIP", zip))
	} else {
		slog.Info("EBAY_ZIP empty, default 94104 (San Francisco) will be used for eBay _stpos")
	}
	ProxyString := os.Getenv("PROXY_URL_LIST")
	if ProxyString == "" {
		slog.Error("proxy string empty")
	} else {
		hostnames := strings.Split(ProxyString, ",")
		seen := make(map[string]bool)
		ProxyList = make([]string, 0, len(hostnames))
		for _, hostname := range hostnames {
			hostname = strings.TrimSpace(hostname)
			if hostname == "" {
				slog.Error("empty proxy string")
				continue
			}
			if seen[hostname] {
				slog.Error("duplicate proxy hostname in PROXY_URL_LIST, exits will share fate",
					slog.String("hostname", hostname))
			}
			seen[hostname] = true
			ProxyList = append(ProxyList, "http://"+hostname+":8888")
		}
		slog.Info("Proxy string and array",
			slog.String("string", ProxyString),
			slog.Any("proxyArr", ProxyList),
		)
		if os.Getenv("WIREGUARD_PRIVATE_KEY_2") == "" {
			slog.Warn("WIREGUARD_PRIVATE_KEY_2 not set: gluetun3 will fail or reuse gluetun2's exit. Generate a 2nd Mullvad device key.")
		}
	}
	ProxyIncidentCounterMap = make(map[string]int)
	ProxySuccessCounterMap = make(map[string]int)
	for _, proxyURL := range ProxyList {
		ProxyIncidentCounterMap[proxyURL] = 0
		ProxySuccessCounterMap[proxyURL] = 0
	}
}

func StartProxyCounter(ctx context.Context) {
	slog.Info("starting proxy counter")
	ProxyCounterChannel = make(chan []*types.Attempt)
	ProxySuccessChannel = make(chan string)
	go func() {
		for {
			select {
			case AttemptArr := <-ProxyCounterChannel:
				slog.Warn("recieved new Attempt Array",
					slog.Any("attemptArr", AttemptArr),
				)
				for i := range AttemptArr {
					if AttemptArr[i].Method == types.MethodChromeDP &&
						AttemptArr[i].Proxy != types.ProxyDisabled {
						slog.Info("chromeDP attempt Found, incrementing counter")
						ProxyIncidentCounterMap[AttemptArr[i].Proxy]++
					}
					if ProxyIncidentCounterMap[AttemptArr[i].Proxy] >= FailureRestartThreshold {
						RestartGluetun(AttemptArr[i].Proxy)
						ProxyIncidentCounterMap[AttemptArr[i].Proxy] = 0
					}
				}
			case ProxyURL := <-ProxySuccessChannel:
				slog.Info("recieved success from proxy success channel",
					slog.String("proxy", ProxyURL),
				)
				ProxySuccessCounterMap[ProxyURL]++
				if ProxySuccessCounterMap[ProxyURL] >= SuccessRestartThreshold {
					slog.Info("reiched Success counter threshold, restarting proxy")
					RestartGluetun(ProxyURL)
					ProxySuccessCounterMap[ProxyURL] = 0
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func RestartGluetun(proxyURL string) error {
	// Step 5: cooldown to avoid hammering the VPN API / churning the
	// same small pool of flagged IPs on every 403 burst.
	if last, ok := lastRestartMap[proxyURL]; ok && time.Since(last) < RestartCooldown {
		slog.Info("skipping gluetun restart, in cooldown",
			slog.String("proxyURL", proxyURL),
			slog.Any("lastRestart", last),
		)
		return nil
	}
	controlURL := strings.Replace(proxyURL, ":8888", ":8000", 1)
	slog.Info("restarting gluetun for proxy",
		slog.String("proxyURL", proxyURL),
		slog.String("controlURL", controlURL),
	)
	client := &http.Client{Timeout: 10 * time.Second}
	for i := 0; i < 3; i++ {
		resp, err := client.Get(controlURL + "/v1/vpn/status")
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

			req, _ := http.NewRequest("PUT", controlURL+"/v1/vpn/status",
				strings.NewReader(`{"status":"stopped"}`))
			req.Header.Set("Content-Type", "application/json")
			client.Do(req)

			time.Sleep(2 * time.Second)

			req, _ = http.NewRequest("PUT", controlURL+"/v1/vpn/status",
				strings.NewReader(`{"status":"running"}`))
			req.Header.Set("Content-Type", "application/json")
			_, err = client.Do(req)
			if err != nil {
				slog.Error("failed to restart VPN", slog.Any("error", err))
				time.Sleep(2 * time.Second)
				continue
			}
			slog.Warn("Gluetun VPN restarted due to incident threshold")
			lastRestartMap[proxyURL] = time.Now()
			return nil
		}

		slog.Info("VPN already stopped, attempting to start")
		req, _ := http.NewRequest("PUT", controlURL+"/v1/vpn/status",
			strings.NewReader(`{"status":"running"}`))
		req.Header.Set("Content-Type", "application/json")
		_, err = client.Do(req)
		if err != nil {
			slog.Error("failed to start VPN", slog.Any("error", err))
			time.Sleep(2 * time.Second)
			continue
		}
		lastRestartMap[proxyURL] = time.Now()
		return nil
	}

	return errors.New("failed to restart VPN after 3 attempts")
}
