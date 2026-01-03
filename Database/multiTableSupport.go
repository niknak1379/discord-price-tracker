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
	Distance  int     `bson:"Distance"`
}

var (
	Tables      = make(map[string]*mongo.Collection)
	Coordinates = make(map[string]Channel)
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
		Coordinates[IDString.ChannelID] = Channel{
			ChannelID: IDString.ChannelID,
			Lat:       IDString.Lat,
			Long:      IDString.Long,
			Distance:  IDString.Distance,
		}
		if IDString.Lat == 0 || IDString.Long == 0 || IDString.Distance == 0 {
			log.Panic("Could not load Channel, lat, long or distance empty")
		}
	}
}

func CreateChannelItemTableIfMissing(ChannelID string, Location string, maxDistance int) error {
	Lat, Long, err := crawler.GetCoordinates(Location)
	if err != nil {
		return err
	}
	Channel := Channel{
		ChannelID: ChannelID,
		Lat:       Lat,
		Long:      Long,
		Distance:  maxDistance,
	}
	// if channelID already exists, just update the Coordinates in DB and memory
	if table, ok := Tables[ChannelID]; ok {
		Coordinates[ChannelID] = Channel
		update := bson.M{
			"$set": bson.M{
				"Distance": maxDistance,
				"Lat":      Lat,
				"Long":     Long,
			},
		}
		Tables[ChannelID] = table
		ChannelTable := Client.Database("tracker").Collection("ChannelIDs")
		ChannelTable.FindOneAndUpdate(ctx, bson.M{"ChannelID": ChannelID}, update)
		return nil
	}
	err = Client.Database("tracker").CreateCollection(context.TODO(), ChannelID)
	if err != nil {
		return err
	}
	// --------------- call to get coordinates goes here --------

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
		return err
	}
	table := Client.Database("tracker").Collection(ChannelID)
	Tables[ChannelID] = table
	Coordinates[ChannelID] = Channel

	return err
}

func ChannelDeleteHandler(ChannelID string) {
	if _, ok := Tables[ChannelID]; ok {
		ChannelTable := Client.Database("tracker").Collection("ChannelIDs")
		ChannelTable.FindOneAndDelete(ctx, bson.M{"ChannelID": ChannelID})
		delete(Tables, ChannelID)
		delete(Coordinates, ChannelID)
	}
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
