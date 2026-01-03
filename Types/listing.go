package types

type EbayListing struct {
	Price     int    `json:"Price"`
	URL       string `json:"URL"`
	Title     string `json:"Title"`
	Condition string `json:"Condition"`
}
