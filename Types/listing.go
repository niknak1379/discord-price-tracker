package types

import "time"

type EbayListing struct {
	ItemName  string    `json:"ItemName"`
	Price     int       `json:"Price"`
	URL       string    `json:"URL"`
	Title     string    `json:"Title"`
	Condition string    `json:"Condition"`
	Date      time.Time `json:"Date"`
}
