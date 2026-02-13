package logger

import (
	"log/slog"

	proxy "priceTracker/Proxy"
	types "priceTracker/Types"
)

var (
	IncidentChannel chan types.Incident
	IncidentCounter int
)

func StartIncidentListener(dbFunc func(*types.Incident), done <-chan struct{}) {
	IncidentChannel = make(chan types.Incident, 100)
	go func() {
		for {
			select {
			case Incident := <-IncidentChannel:
				slog.Warn("logging Incident", slog.Any("incident", Incident))
				dbFunc(&Incident)
				proxy.ProxyCounterChannel <- Incident.Attempts
			case <-done:
				return
			}
		}
	}()
}
