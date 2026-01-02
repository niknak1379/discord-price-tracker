package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	crawler "priceTracker/Crawler"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Channel struct {
	ChannelID string  `bson:"ChannelID"`
	Lat       float64 `bson:"Lat"`
	Long      float64 `bson:"Long"`
}
type ChannelCoord struct {
	Lat  float64 `bson:"Lat"`
	Long float64 `bson:"Long"`
}

var (
	Tables      = make(map[string]*mongo.Collection)
	Coordinates = make(map[string]ChannelCoord)
	logger      = slog.New(slog.NewJSONHandler(os.Stdout, nil))
)

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
		Coordinates[IDString.ChannelID] = ChannelCoord{
			Lat:  IDString.Lat,
			Long: IDString.Long,
		}
	}
}

func createChannelItemTableIfMissing(ChannelID string, Location string) *mongo.Collection {
	err := Client.Database("tracker").CreateCollection(context.TODO(), ChannelID)
	if err != nil {
		log.Fatalf("Failed to create collection: %v", err)
	}
	// --------------- call to get coordinates goes here --------
	Lat, Long, err := crawler.GetCoordinates(Location)
	Channel := Channel{
		Lat:       Lat,
		Long:      Long,
		ChannelID: ChannelID,
	}
	Client.Database("tracker").Collection("ChannelIDs").InsertOne(ctx, Channel)

	Table := Client.Database("tracker").Collection(ChannelID)
	// Sets the index name and type to "search"
	opts := options.SearchIndexes().SetName(ChannelID).SetType("search")
	// Defines the index definition
	searchIndexModel := mongo.SearchIndexModel{
		Definition: bson.D{
			{Key: "mappings", Value: bson.D{
				{Key: "dynamic", Value: false},
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
	Tables[ChannelID] = Channel
	Coordinates[Channel.ChannelID] = ChannelCoord{
		Lat:  Channel.Lat,
		Long: Channel.Long,
	}

	return Table
}

func loadChannelTable(ChannelID string) (*mongo.Collection, error) {
	Table, ok := Tables[ChannelID]
	if !ok {
		fmt.Println("channel does not exist have to call setup first")
		//<------ make this a specific error that propogates 
		// that forces the crawl thing to send error?
		err := errors.New("channel not found in db, call setup function first")
		return Table, err
	}
	return Table, nil
}
