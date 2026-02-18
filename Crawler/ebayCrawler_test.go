package crawler

import (
	"testing"
	"time"
)

func TestBidProcessingFunction(t *testing.T) {
	tests := []struct {
		Name         string
		TextInput    string
		TimeInput    time.Time
		ExpectedBid  int
		ExpectedTime time.Time
	}{
		{
			Name:         "Empty String Case",
			TextInput:    "",
			TimeInput:    time.Time{},
			ExpectedBid:  0,
			ExpectedTime: time.Time{},
		},
		{
			Name:         "Bid String with days and hours",
			TextInput:    "34 bids · Time left2d 23h left",
			TimeInput:    time.Time{},
			ExpectedBid:  34,
			ExpectedTime: time.Time{}.Add((2*24 + 23) * time.Hour),
		},
		{
			Name:         "Bid String with hours and minutes",
			TextInput:    "1 bid · Time left2h 23m left",
			TimeInput:    time.Time{},
			ExpectedBid:  1,
			ExpectedTime: time.Time{}.Add((2)*time.Hour + 23*time.Minute),
		},
		{
			Name:         "Bid String with only minutes",
			TextInput:    "12 bids · Time left37m left (Today 02:30 PM)",
			TimeInput:    time.Time{},
			ExpectedBid:  12,
			ExpectedTime: time.Time{}.Add((37) * time.Minute),
		},
		{
			Name:         "Bid String with minutes and seconds",
			TextInput:    "12 bids · Time left5m 8s left (Today 02:30 PM)",
			TimeInput:    time.Time{},
			ExpectedBid:  12,
			ExpectedTime: time.Time{}.Add(8*time.Second + 5*time.Minute),
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.Name, func(t *testing.T) {
			resBid, resTime := ProcessBidRawString(testCase.TextInput, testCase.TimeInput)
			if resBid != testCase.ExpectedBid {
				t.Errorf("Expected bid: %d, returend bid: %d", testCase.ExpectedBid, resBid)
			}
			if resTime != testCase.ExpectedTime {
				t.Errorf("Expected time: %v, returned time %v", testCase.ExpectedTime, resTime)
			}
		})
	}
}
