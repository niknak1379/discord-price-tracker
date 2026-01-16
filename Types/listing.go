package types

import "time"

type EbayListing struct {
	ItemName         string        `bson:"ItemName"`
	Price            int           `bson:"Price"`
	URL              string        `bson:"URL"`
	Duration         time.Duration `bson:"Duration"`
	Title            string        `bson:"Title"`
	Condition        string        `bson:"Condition"`
	Date             time.Time     `bson:"Date"`
	PriceIncreaseNum int           `bson:"PriceIncreaseNum"`
	PriceDecreaseNum int           `bson:"PriceDecreaseNum"`
}
