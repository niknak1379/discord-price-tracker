package scheduler

import (
	"log/slog"

	database "priceTracker/Database"
)

type IncidentRate struct {
	// maps url to number of failures
	DefaultTrackers map[string]int
	EbayTracker     int
	FacebookTracker int
	DepopTracker    int
}

var ErrorLimit = 2

// HaveItemPropertiesChanged checks if any item properties have changed between two versions.
//
// Parameters:
//   - currItem: the current item state
//   - oldItem: the previous item state
//
// Returns true if any properties have changed.
func HaveItemPropertiesChanged(currItem, oldItem *database.Item) bool {
	if currItem.SuppressNotifications != oldItem.SuppressNotifications ||
		currItem.Timer != oldItem.Timer ||
		len(currItem.AlternateTrackingQueries) != len(oldItem.AlternateTrackingQueries) ||
		len(currItem.TrackingExclusionQueries) != len(oldItem.TrackingExclusionQueries) ||
		currItem.CurrentLowestPrice.Price != oldItem.CurrentLowestPrice.Price ||
		currItem.FacebookCrawl != oldItem.FacebookCrawl ||
		currItem.SecondHandPrice != oldItem.SecondHandPrice {
		return true
	}
	// check weather tracking list was changed
	if len(currItem.TrackingList) == len(oldItem.TrackingList) {
		for index := range currItem.TrackingList {
			if currItem.TrackingList[index].HtmlQuery != oldItem.TrackingList[index].HtmlQuery ||
				currItem.TrackingList[index].URI != oldItem.TrackingList[index].URI {
				return true
			}
		}
	} else {
		return true
	}
	return false
}

func HasIncidentRateLimitReached(I *IncidentRate) bool {
	slog.Warn("checking Incident Rate due to error")
	for URI, ErrorFreq := range I.DefaultTrackers {
		if ErrorFreq == ErrorLimit {
			slog.Warn("Incident Rate exceeded for uri: ",
				slog.String("URI", URI),
			)
			return true
		}
	}
	if I.EbayTracker == ErrorLimit {
		slog.Warn("Incident Rate Exceeded for Ebay")
		I.EbayTracker = 0
		return true
	}
	if I.FacebookTracker == ErrorLimit {
		slog.Warn("Incident Rate Exceeded for Facebook")
		I.FacebookTracker = 0
		return true
	}
	if I.DepopTracker == ErrorLimit {
		slog.Warn("Incident Rate Exceeded for Depop")
		I.DepopTracker = 0
		return true
	}
	slog.Warn("Incident Rate Not Exceeded Yet")
	return false
}

func initIncidentRate(Item *database.Item) *IncidentRate {
	I := IncidentRate{
		EbayTracker:     0,
		FacebookTracker: 0,
		DepopTracker:    0,
	}
	TrackerMap := make(map[string]int)
	for _, tracker := range Item.TrackingList {
		TrackerMap[tracker.URI] = 0
	}
	I.DefaultTrackers = TrackerMap
	return &I
}
