package types

type EbayListing struct {
	Price     int    `json:"Price"`
	URL       string `json:"URL"`
	Title     string `json:"Title"`
	Condition string `json:"Condition"`
}

type Channel struct {
	ChannelID string  `bson:"ChannelID"`
	Lat       float64 `bson:"Lat"`
	Long      float64 `bson:"Long"`
}
