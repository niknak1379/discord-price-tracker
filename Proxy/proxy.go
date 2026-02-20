package proxy

import (
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
	ProxyList               []string
	ProxyIncidentCounterMap map[string]int
	ProxyCounterChannel     chan []*types.Attempt
	ProxySuccessChannel     chan string
	ProxySuccessCounterMap  map[string]int
)

func Init() {
	slog.Info("initializeing proxy rotator module")
	ProxyString := os.Getenv("PROXY_URL_LIST")
	if ProxyString == "" {
		slog.Error("proxy string empty")
	} else {
		hostnames := strings.Split(ProxyString, ",")
		ProxyList = make([]string, len(hostnames))
		for i, hostname := range hostnames {
			hostname = strings.TrimSpace(hostname)
			if hostname == "" {
				slog.Error("empty proxy string")
				continue
			}
			ProxyList[i] = "http://" + hostname + ":8888"
		}
		slog.Info("Proxy string and array",
			slog.String("string", ProxyString),
			slog.Any("proxyArr", ProxyList),
		)
	}
	ProxyIncidentCounterMap = make(map[string]int)
	ProxySuccessCounterMap = make(map[string]int)
	for _, proxyURL := range ProxyList {
		ProxyIncidentCounterMap[proxyURL] = 0
		ProxySuccessCounterMap[proxyURL] = 0
	}
}

func StartProxyCounter(done <-chan struct{}) {
	slog.Info("starting proxy counter")
	ProxyCounterChannel = make(chan []*types.Attempt, 100)
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
			case <-done:
				return
			}
		}
	}()
}

func RestartGluetun(proxyURL string) error {
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
		return nil
	}

	return errors.New("failed to restart VPN after 3 attempts")
}
