package database

import (
	"log"
	"log/slog"
	logger "priceTracker/Logger"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)
func FuzzyMatchName(Name string, ChannelID string) []string {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
		return make([]string, 0)
	}

	projectStage := bson.D{{Key: "$project", Value: bson.D{{Key: "Name", Value: 1}}}}
	var pipeline mongo.Pipeline

	if Name == "" {
		pipeline = mongo.Pipeline{
			bson.D{{Key: "$match", Value: bson.D{}}},
			bson.D{{Key: "$sort", Value: bson.D{{Key: "Name", Value: 1}}}},
			projectStage,
		}
	} else {
		pipeline = mongo.Pipeline{
			bson.D{{Key: "$search", Value: bson.D{
				{Key: "index", Value: ChannelID},
				{Key: "autocomplete", Value: bson.D{
					{Key: "path", Value: "Name"},
					{Key: "query", Value: Name},
					{Key: "fuzzy", Value: bson.D{
						{Key: "maxEdits", Value: 2},
						{Key: "prefixLength", Value: 1},
					}},
				}},
			}}},
			projectStage,
		}
	}

	cursor, err := Table.Aggregate(ctx, pipeline)
	if err != nil {
		logger.Logger.Error("Error", slog.Any("Error", err))
	}
	defer cursor.Close(ctx)
	names := make([]string, 0)

	for cursor.Next(ctx) {
		var result bson.M
		if err := cursor.Decode(&result); err != nil {
			logger.Logger.Error("Error", slog.Any("Error", err))
			continue
		}

		if name, ok := result["Name"].(string); ok {
			names = append(names, name)
		}
	}

	if err := cursor.Err(); err != nil {
		logger.Logger.Error("Error", slog.Any("Error", err))
	}

	return names
}

// not really critical functionality i feel like i dont really
// need to propogate the errors for this and the other autocomplete
func AutoCompleteURL(Name string, ChannelID string) []string {
	item, err := GetItem(Name, ChannelID)
	res := []string{}
	if err != nil {
		return res
	}
	for _, tracker := range item.TrackingList {
		res = append(res, tracker.URI)
	}
	return res
}

func AutoCompleteQuery() map[string]string {
	ret := map[string]string{
		"Amazon":       "form#addToCart span.a-price-whole",
		"NewEgg":       "div.price-current>strong",
		"MicroCenter":  "#options-pricing2022",
		"BHPhotoVideo": "span[class^='price_']",
		"BestBuy": "div[data-testid='price-block-customer-price']",
	}
	return ret
}
