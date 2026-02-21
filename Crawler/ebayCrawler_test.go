package crawler

import (
	"os"
	"testing"
	"time"

	types "priceTracker/Types"
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
		{
			Name:         "Bid String with day only",
			TextInput:    "12 bids · Time left2d left (Today 02:30 PM)",
			TimeInput:    time.Time{},
			ExpectedBid:  12,
			ExpectedTime: time.Time{}.Add(2 * 24 * time.Hour),
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

func TestHTMLProcessingFunction(t *testing.T) {
	tests := []struct {
		Name            string
		AdditionalNames []string
		ExclusionNames  []string
		DesiredPrice    int
		HTMLFilePath    string
		ExpectedListing []*types.EbayListing
		ExpectedBids    []*types.EbayBids
	}{{
		Name:            "Samsung odyssey G80sd",
		AdditionalNames: []string{},
		ExclusionNames:  []string{},
		DesiredPrice:    1000,
		HTMLFilePath:    "testdata/listingTest1",
		ExpectedListing: []*types.EbayListing{
			{
				ItemName:      "Samsung odyssey G80sd",
				Title:         "SAMSUNG 32\" Odyssey G80SD 4K UHD Smart Gaming MonitorLS32D802UANXGO",
				Price:         302,
				URL:           "https://www.ebay.com/itm/277491998203",
				Condition:     "Pre-Owned",
				AcceptsOffers: true,
			},
		},
		ExpectedBids: []*types.EbayBids{},
	}}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			InitAntiTLSClients()
			queries := InitTitleRegex(append(test.AdditionalNames,
				test.Name), test.ExclusionNames,
			)
			HTMLInput := loadTestFile(t, test.HTMLFilePath)
			listings, bids, err := ParseChromedpHTML(HTMLInput,
				test.DesiredPrice, test.Name, *queries)
			if err != nil {
				t.Errorf("Did not expect Error, recieved: %v", err)
			}
			if len(listings) != len(test.ExpectedListing) {
				t.Errorf("Listing count mismatch: expected %d, got %d",
					len(test.ExpectedListing), len(listings))
				return
			}
			for i := range listings {
				compareEbayListings(t, i, test.ExpectedListing[i], listings[i])
			}
			if len(bids) != len(test.ExpectedBids) {
				t.Errorf("Bids count mismatch: expected %d, got %d",
					len(test.ExpectedBids), len(bids))
				return
			}
			for i := range bids {
				compareEbayBids(t, i, test.ExpectedBids[i], bids[i])
			}
		})
	}
}

func loadTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to load fixture: %v", err)
	}
	return string(data)
}

func compareEbayListings(t *testing.T, index int, expected, actual *types.EbayListing) {
	if expected.ItemName != actual.ItemName {
		t.Errorf("Listing[%d].ItemName mismatch: expected=%q, got=%q",
			index, expected.ItemName, actual.ItemName)
	}
	if expected.Price != actual.Price {
		t.Errorf("Listing[%d].Price mismatch: expected=%d, got=%d",
			index, expected.Price, actual.Price)
	}
	if expected.URL != actual.URL {
		t.Errorf("Listing[%d].URL mismatch: expected=%q, got=%q",
			index, expected.URL, actual.URL)
	}
	if expected.Duration != actual.Duration {
		t.Errorf("Listing[%d].Duration mismatch: expected=%v, got=%v",
			index, expected.Duration, actual.Duration)
	}
	if expected.Title != actual.Title {
		t.Errorf("Listing[%d].Title mismatch: expected=%q, got=%q",
			index, expected.Title, actual.Title)
	}
	if expected.Condition != actual.Condition {
		t.Errorf("Listing[%d].Condition mismatch: expected=%q, got=%q",
			index, expected.Condition, actual.Condition)
	}
	if expected.PriceIncreaseNum != actual.PriceIncreaseNum {
		t.Errorf("Listing[%d].PriceIncreaseNum mismatch: expected=%d, got=%d",
			index, expected.PriceIncreaseNum, actual.PriceIncreaseNum)
	}
	if expected.PriceDecreaseNum != actual.PriceDecreaseNum {
		t.Errorf("Listing[%d].PriceDecreaseNum mismatch: expected=%d, got=%d",
			index, expected.PriceDecreaseNum, actual.PriceDecreaseNum)
	}
	if expected.TotalPriceChange != actual.TotalPriceChange {
		t.Errorf("Listing[%d].TotalPriceChange mismatch: expected=%d, got=%d",
			index, expected.TotalPriceChange, actual.TotalPriceChange)
	}
	if expected.AcceptsOffers != actual.AcceptsOffers {
		t.Errorf("Listing[%d].AcceptsOffers mismatch: expected=%v, got=%v",
			index, expected.AcceptsOffers, actual.AcceptsOffers)
	}
}

func compareEbayBids(t *testing.T, index int, expected, actual *types.EbayBids) {
	if expected.ItemName != actual.ItemName {
		t.Errorf("Bid[%d].ItemName mismatch: expected=%q, got=%q",
			index, expected.ItemName, actual.ItemName)
	}
	if expected.Title != actual.Title {
		t.Errorf("Bid[%d].Title mismatch: expected=%q, got=%q",
			index, expected.Title, actual.Title)
	}
	if expected.Condition != actual.Condition {
		t.Errorf("Bid[%d].Condition mismatch: expected=%q, got=%q",
			index, expected.Condition, actual.Condition)
	}
	if expected.Price != actual.Price {
		t.Errorf("Bid[%d].Price mismatch: expected=%d, got=%d",
			index, expected.Price, actual.Price)
	}
	if expected.URL != actual.URL {
		t.Errorf("Bid[%d].URL mismatch: expected=%q, got=%q",
			index, expected.URL, actual.URL)
	}
	if expected.Bids != actual.Bids {
		t.Errorf("Bid[%d].Bids mismatch: expected=%d, got=%d",
			index, expected.Bids, actual.Bids)
	}
	if !expected.EndDate.Equal(actual.EndDate) {
		t.Errorf("Bid[%d].EndDate mismatch: expected=%v, got=%v",
			index, expected.EndDate, actual.EndDate)
	}
}
