package database

import (
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// GetPriceHistory retrieves price history for an item from a specific date.
//
// Parameters:
//   - Name: the item name
//   - date: the start date for retrieving history
//   - ChannelID: the Discord channel ID
//
// Returns the price history and any error encountered.
func GetPriceHistory(Name string, date time.Time, ChannelID string) ([]*Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("Could not load Channel from DB", slog.Any("Error", err))
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
		slog.Error("Error aggregating price history", slog.Any("Error", err))
		return newRes, err
	}
	if err = cursor.All(ctx, &newRes); err != nil {
		slog.Error("Error getting price history from cursor", slog.Any("Error", err))
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
										6,
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
		slog.Error("error aggregating price history", slog.Any("Error", err))
		return newRes, err
	}
	if err = cursor.All(ctx, &usedAvgRes); err != nil {
		slog.Error("Error getting from cursor", slog.Any("Error", err))
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
										6,
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
				{Key: "Url", Value: "USED-LOWEST"},
			}},
		},
	}
	cursor, err = Table.Aggregate(ctx, usedLowestPipeline)
	if err != nil {
		slog.Error("couldnt aggregate", slog.Any("Error", err))
		return newRes, err
	}
	if err = cursor.All(ctx, &usedLowestRes); err != nil {
		slog.Error("couldnt aggregate", slog.Any("Error", err))
		return newRes, err
	}
	defer cursor.Close(ctx)

	return append(newRes, usedLowestRes...), err
}

// GenerateSecondHandPriceReport generates aggregate statistics for second-hand listings.
//
// Parameters:
//   - Name: the item name
//   - endDate: the end date for the report period
//   - Days: the number of days to include in the report
//   - ChannelID: the Discord channel ID
//
// Returns the aggregate report and any error encountered.
func GenerateSecondHandPriceReport(Name string, endDate time.Time, Days int, ChannelID string) (AggregateReport, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt aggregate", slog.Any("Error", err))
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
				{Key: "_id", Value: bson.D{
					{Key: "$dateTrunc", Value: bson.D{
						{Key: "date", Value: "$Date"},
						{Key: "unit", Value: "day"},
					}},
				}},
				{Key: "AVGPrice", Value: bson.D{{Key: "$avg", Value: "$Price"}}},
				{Key: "STDEV", Value: bson.D{{Key: "$stdDevPop", Value: "$Price"}}},
				{Key: "ListingsHistory", Value: bson.D{{Key: "$push", Value: "$$ROOT"}}},
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
				{Key: "_id", Value: "$ListingsHistory.URL"},
				{Key: "first", Value: bson.D{{Key: "$min", Value: "$ListingsHistory.Date"}}},
				{Key: "last", Value: bson.D{{Key: "$max", Value: "$ListingsHistory.Date"}}},
				{Key: "priceWhenSold", Value: bson.D{{Key: "$last", Value: "$ListingsHistory.Price"}}},
				{Key: "averagePrice", Value: bson.D{{Key: "$avg", Value: "$ListingsHistory.Price"}}},
				{Key: "LowestPriceDuringTimePeriod", Value: bson.D{{Key: "$min", Value: "$ListingsHistory.Price"}}},
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
				{Key: "last", Value: "$last"},
				{Key: "priceWhenSold", Value: "$priceWhenSold"},
				{Key: "averagePrice", Value: "$averagePrice"},
				{Key: "LowestPriceDuringTimePeriod", Value: "$LowestPriceDuringTimePeriod"},
			}},
		},
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: nil},
				{Key: "AverageDaysUP", Value: bson.D{{Key: "$avg", Value: "$DaysUp"}}},
				{Key: "AveragePriceWhenSold", Value: bson.D{
					{Key: "$avg", Value: bson.D{
						{Key: "$cond", Value: bson.A{
							bson.D{{Key: "$lt", Value: bson.A{"$last", endDate}}},
							"$priceWhenSold",
							nil,
						}},
					}},
				}},
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
		slog.Error("couldnt aggregate", slog.Any("Error", err))
		return AggregateReport{}, err
	}
	if err = cursor.All(ctx, &res); err != nil {
		slog.Error("couldnt aggregate", slog.Any("Error", err))
		return AggregateReport{}, err
	}
	if len(res) == 0 {
		return AggregateReport{}, err
	}
	return *res[0], err
}

// UpdateAggregateReport updates the 7-day aggregate report for an item.
//
// Parameters:
//   - Name: the item name
//   - ChannelID: the Discord channel ID
//
// Returns any error encountered.
func UpdateAggregateReport(Name, ChannelID string) error {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load table", slog.Any("Error", err))
		return err
	}
	AggregateReport, err := GenerateSecondHandPriceReport(Name, time.Now(), 7, ChannelID)
	if err != nil {
		slog.Error("failed to get second hand reports for",
			slog.Any("error value", err),
			slog.String("Title", Name),
		)
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
		slog.Error("could not update new aggregate",
			slog.Any("value", err))
		return err
	}
	return nil
}

func GetIncidentsByDomainOverTime(startDate, endDate time.Time) ([]DomainTimeSeries, error) {
	collection := Client.Database("tracker").Collection("Incidents")
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "StartTime", Value: bson.D{
				{Key: "$gte", Value: startDate},
				{Key: "$lte", Value: endDate},
			}},
		}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "Domain", Value: "$Domain"},
				{Key: "Date", Value: bson.D{
					{Key: "$dateTrunc", Value: bson.D{
						{Key: "date", Value: "$StartTime"},
						{Key: "unit", Value: "day"},
					}},
				}},
			}},
			{Key: "Count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "Domain", Value: "$_id.Domain"},
			{Key: "Date", Value: "$_id.Date"},
			{Key: "Count", Value: 1},
			{Key: "_id", Value: 0},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "Date", Value: 1},
			{Key: "Domain", Value: 1},
		}}},
	}

	var results []DomainTimeSeries
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		slog.Error("failed to aggregate incidents by domain over time", slog.Any("error", err))
		return nil, err
	}
	if err := cursor.All(ctx, &results); err != nil {
		slog.Error("failed to decode incidents by domain over time", slog.Any("error", err))
		return nil, err
	}
	return results, nil
}

func GetIncidentsByDomainMethodProxy(startDate, endDate time.Time) ([]IncidentTimeSeries, error) {
	collection := Client.Database("tracker").Collection("Incidents")
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "StartTime", Value: bson.D{
				{Key: "$gte", Value: startDate},
				{Key: "$lte", Value: endDate},
			}},
		}}},
		bson.D{{Key: "$unwind", Value: "$Attempts"}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "Domain", Value: "$Domain"},
				{Key: "Method", Value: "$Attempts.Method"},
				{Key: "Proxy", Value: "$Attempts.Proxy"},
				{Key: "Date", Value: bson.D{
					{Key: "$dateTrunc", Value: bson.D{
						{Key: "date", Value: "$StartTime"},
						{Key: "unit", Value: "day"},
					}},
				}},
			}},
			{Key: "Count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "Domain", Value: "$_id.Domain"},
			{Key: "Method", Value: "$_id.Method"},
			{Key: "Proxy", Value: "$_id.Proxy"},
			{Key: "Date", Value: "$_id.Date"},
			{Key: "Count", Value: 1},
			{Key: "_id", Value: 0},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{
			{Key: "Date", Value: 1},
			{Key: "Domain", Value: 1},
			{Key: "Method", Value: 1},
			{Key: "Proxy", Value: 1},
		}}},
	}

	var results []IncidentTimeSeries
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		slog.Error("failed to aggregate incidents by domain method proxy", slog.Any("error", err))
		return nil, err
	}
	if err := cursor.All(ctx, &results); err != nil {
		slog.Error("failed to decode incidents by domain method proxy", slog.Any("error", err))
		return nil, err
	}
	return results, nil
}
