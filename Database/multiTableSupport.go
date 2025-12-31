package database

import (
	"context"
	"log"
	"log/slog"
	"os"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	Tables = make(map[string]*mongo.Collection)
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
)

type Channel struct {
	ChannelID string `bson:"ChannelID"`
}

func loadDBTables() {
	var ChannelsArr []Channel
	ChannelTable := Client.Database("tracker").Collection("ChannelIDs")
	cursor, err := ChannelTable.Find(ctx, bson.M{})
	if err != nil {
		log.Panic("could not load database tables")
	}
	err = cursor.All(ctx, &ChannelsArr)
	if err != nil {
		log.Panic("could not read ChannelID results")
	}
	logger.Info("channels", slog.Any("IDs:", ChannelsArr))
	for _, IDString := range ChannelsArr {
		table := Client.Database("tracker").Collection(IDString.ChannelID)
		Tables[IDString.ChannelID] = table
	}
}

func createChannelItemTableIfMissing(ChannelID string) *mongo.Collection {
	err := Client.Database("tracker").CreateCollection(context.TODO(), ChannelID)
	if err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	}
	Table := Client.Database("tracker").Collection(ChannelID)
	// Sets the index name and type to "search"
	const indexName = "default"
	opts := options.SearchIndexes().SetName(indexName).SetType("search")
	// Defines the index definition
	searchIndexModel := mongo.SearchIndexModel{
		Definition: bson.D{
			{Key: "mappings", Value: bson.D{
				{Key: "dynamic", Value: true},
				{Key: "fields", Value: bson.D{
					{Key: "Name", Value: bson.D{
						{Key: "type", Value: "autocomplete"},
					}},
				}},
			}},
		},
		Options: opts,
	}
	// Creates the index
	_, err = Table.SearchIndexes().CreateOne(ctx, searchIndexModel)
	if err != nil {
		log.Fatalf("Failed to create the MongoDB Search index: %v", err)
	}
	return Table
}

func loadChannelTable(ChannelID string) *mongo.Collection {
	Table, ok := Tables[ChannelID]
	if !ok {
		Table = createChannelItemTableIfMissing(ChannelID)
	}
	return Table
}
