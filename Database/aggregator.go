package database

import (
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func GetPriceHistory(Name string, date time.Time, ChannelID string) ([]*Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB", err)
		return []*Price{}, err
	}
	var newRes []*Price
	// ------------ pipeline for getting New Price -------------
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "Name", Value: bson.D{
				{Key: "$regex", Value: "^" + Name + "$"},
				{Key: "$options", Value: "i"},
			}},
		}}},
		bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$PriceHistory"}}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "PriceHistory.Date", Value: 1}}}},
		bson.D{
			{
				Key: "$project",
				Value: bson.D{
					{Key: "Date", Value: "$PriceHistory.Date"},
					{Key: "Price", Value: "$PriceHistory.Price"},
					{Key: "Url", Value: "$PriceHistory.Url"},
				},
			},
		},
	}
	cursor, err := Table.Aggregate(ctx, pipeline)
	if err != nil {
		log.Print("error aggregating price history", err)
		return newRes, err
	}
	if err = cursor.All(ctx, &newRes); err != nil {
		log.Print("error aggregating price history from cursor", err)
		return newRes, err
	}

	// ------------ pipeline for getting used Price -------------
	usedAvgRes := []*Price{}
	usedAvgPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "Name", Value: bson.D{
				{Key: "$regex", Value: "^" + Name + "$"},
				{Key: "$options", Value: "i"},
			}},
		}}},
		bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$ListingsHistory"}}}},
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{
					{Key: "$dateTrunc", Value: bson.D{
						{Key: "date", Value: "$ListingsHistory.Date"},
						{Key: "unit", Value: "day"},
					}},
				}},
				{Key: "AVGPrice", Value: bson.D{{Key: "$avg", Value: "$ListingsHistory.Price"}}},
				{Key: "STDEV", Value: bson.D{{Key: "$stdDevPop", Value: "$ListingsHistory.Price"}}},
				{Key: "ListingsHistory", Value: bson.D{{Key: "$push", Value: "$ListingsHistory"}}},
			}},
		},
		bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$ListingsHistory"}}}},
		bson.D{
			{Key: "$match", Value: bson.D{
				{Key: "$expr", Value: bson.D{
					{Key: "$gte", Value: bson.A{
						"$ListingsHistory.Price",
						bson.D{
							{Key: "$subtract", Value: bson.A{
								"$AVGPrice",
								bson.D{
									{Key: "$multiply", Value: bson.A{
										"$STDEV",
										3,
									}},
								},
							}},
						},
					}},
				}},
			}},
		},
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{
					{Key: "$dateTrunc", Value: bson.D{
						{Key: "date", Value: "$ListingsHistory.Date"},
						{Key: "unit", Value: "day"},
					}},
				}},
				{Key: "Price", Value: bson.D{{Key: "$avg", Value: "$ListingsHistory.Price"}}},
			}},
		},
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "Price", Value: bson.D{{Key: "$toInt", Value: "$Price"}}},
				{Key: "Date", Value: "$_id"},
				{Key: "Url", Value: "USED"},
			}},
		},
	}
	cursor, err = Table.Aggregate(ctx, usedAvgPipeline)
	if err != nil {
		log.Print("error aggregating used avg price history", err)
		return newRes, err
	}
	if err = cursor.All(ctx, &usedAvgRes); err != nil {
		log.Print("error aggregating used avg price history from cursor", err)
		return newRes, err
	}
	newRes = append(newRes, usedAvgRes...)
	var usedLowestRes []*Price
	usedLowestPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "Name", Value: bson.D{
				{Key: "$regex", Value: "^" + Name + "$"},
				{Key: "$options", Value: "i"},
			}},
		}}},
		bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$ListingsHistory"}}}},
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{
					{Key: "$dateTrunc", Value: bson.D{
						{Key: "date", Value: "$ListingsHistory.Date"},
						{Key: "unit", Value: "day"},
					}},
				}},
				{Key: "AVGPrice", Value: bson.D{{Key: "$avg", Value: "$ListingsHistory.Price"}}},
				{Key: "STDEV", Value: bson.D{{Key: "$stdDevPop", Value: "$ListingsHistory.Price"}}},
				{Key: "ListingsHistory", Value: bson.D{{Key: "$push", Value: "$ListingsHistory"}}},
			}},
		},
		bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$ListingsHistory"}}}},
		bson.D{
			{Key: "$match", Value: bson.D{
				{Key: "$expr", Value: bson.D{
					{Key: "$gte", Value: bson.A{
						"$ListingsHistory.Price",
						bson.D{
							{Key: "$subtract", Value: bson.A{
								"$AVGPrice",
								bson.D{
									{Key: "$multiply", Value: bson.A{
										"$STDEV",
										3,
									}},
								},
							}},
						},
					}},
				}},
			}},
		},
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{
					{Key: "$dateTrunc", Value: bson.D{
						{Key: "date", Value: "$ListingsHistory.Date"},
						{Key: "unit", Value: "day"},
					}},
				}},
				{Key: "Price", Value: bson.D{{Key: "$min", Value: "$ListingsHistory.Price"}}},
			}},
		},
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "Price", Value: bson.D{{Key: "$toInt", Value: "$Price"}}},
				{Key: "Date", Value: "$_id"},
				{Key: "Url", Value: "USED_LOWEST"},
			}},
		},
	}
	cursor, err = Table.Aggregate(ctx, usedLowestPipeline)
	if err != nil {
		log.Print("error aggregating used min price history", err)
		return newRes, err
	}
	if err = cursor.All(ctx, &usedLowestRes); err != nil {
		log.Print("error aggregating used min price history from cursor", err)
		return newRes, err
	}
	defer cursor.Close(ctx)

	return append(newRes, usedLowestRes...), err
}

func GenerateSecondHandPriceReport(Name string, endDate time.Time, Days int, ChannelID string) (AggregateReport, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB", err)
		return AggregateReport{}, err
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "Name", Value: Name}}}},
		bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$ListingsHistory"}}}},
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "URL", Value: "$ListingsHistory.URL"},
				{Key: "Date", Value: "$ListingsHistory.Date"},
				{Key: "Price", Value: "$ListingsHistory.Price"},
			}},
		},
		bson.D{
			{Key: "$match", Value: bson.D{
				{Key: "$and", Value: bson.A{
					bson.D{{Key: "Date", Value: bson.D{{Key: "$gte", Value: endDate.AddDate(0, 0, -1*Days)}}}},
					bson.D{{Key: "Date", Value: bson.D{{Key: "$lte", Value: endDate}}}},
				}},
			}},
		},
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$URL"},
				{Key: "first", Value: bson.D{{Key: "$min", Value: "$Date"}}},
				{Key: "last", Value: bson.D{{Key: "$max", Value: "$Date"}}},
				{Key: "priceWhenSold", Value: bson.D{{Key: "$last", Value: "$Price"}}},
				{Key: "averagePrice", Value: bson.D{{Key: "$avg", Value: "$Price"}}},
				{Key: "LowestPriceDuringTimePeriod", Value: bson.D{{Key: "$min", Value: "$Price"}}},
			}},
		},
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "DaysUp", Value: bson.D{
					{Key: "$dateDiff", Value: bson.D{
						{Key: "startDate", Value: "$first"},
						{Key: "endDate", Value: "$last"},
						{Key: "unit", Value: "day"},
						{Key: "timezone", Value: "America/Los_Angeles"},
					}},
				}},
				{Key: "priceWhenSold", Value: "$priceWhenSold"},
				{Key: "averagePrice", Value: "$averagePrice"},
				{Key: "LowestPriceDuringTimePeriod", Value: "$LowestPriceDuringTimePeriod"},
			}},
		},
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: nil},
				{Key: "AverageDaysUP", Value: bson.D{{Key: "$avg", Value: "$DaysUp"}}},
				{Key: "AveragePriceWhenSold", Value: bson.D{{Key: "$avg", Value: "$priceWhenSold"}}},
				{Key: "AveragePrice", Value: bson.D{{Key: "$avg", Value: "$averagePrice"}}},
				{Key: "PriceSTDEV", Value: bson.D{{Key: "$stdDevSamp", Value: "$LowestPriceDuringTimePeriod"}}},
				{Key: "UniqueListings", Value: bson.D{{Key: "$sum", Value: 1}}},
				{Key: "LowestPriceDuringTimePeriod", Value: bson.D{{Key: "$min", Value: "$LowestPriceDuringTimePeriod"}}},
			}},
		},
		bson.D{
			{Key: "$project", Value: bson.D{
				{Key: "AverageDaysUP", Value: bson.D{{Key: "$toInt", Value: "$AverageDaysUP"}}},
				{Key: "AveragePriceWhenSold", Value: bson.D{{Key: "$toInt", Value: "$AveragePriceWhenSold"}}},
				{Key: "AveragePrice", Value: bson.D{{Key: "$toInt", Value: "$AveragePrice"}}},
				{Key: "PriceSTDEV", Value: bson.D{{Key: "$toInt", Value: "$PriceSTDEV"}}},
				{Key: "UniqueListings", Value: "$UniqueListings"},
				{Key: "LowestPriceDuringTimePeriod", Value: bson.D{{Key: "$toInt", Value: "$LowestPriceDuringTimePeriod"}}},
			}},
		},
	}
	var res []*AggregateReport
	cursor, err := Table.Aggregate(ctx, pipeline)
	if err != nil {
		log.Print("error aggregate report from runnign pipeline", err)
		return AggregateReport{}, err
	}
	if err = cursor.All(ctx, &res); err != nil {
		log.Print("error aggregate report after decode", err)
		return AggregateReport{}, err
	}
	if len(res) == 0 {
		return AggregateReport{}, err
	}
	return *res[0], err
}

func UpdateAggregateReport(Name, ChannelID string) error {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
		return err
	}
	AggregateReport, err := GenerateSecondHandPriceReport(Name, time.Now(), 7, ChannelID)
	if err != nil {
		fmt.Println("error getting aggregate from generate", err)
		return err
	}
	result := Table.FindOneAndUpdate(ctx, bson.M{
		"Name": Name,
	}, bson.M{
		"$set": bson.M{
			"SevenDayAggregate": AggregateReport,
		},
	})
	if result.Err() != nil {
		fmt.Println("could not update item with new aggregate", err)
		return err
	}
	return nil
}

