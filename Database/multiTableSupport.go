package database

import (
	"context"
	"log"
	"log/slog"
	"os"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
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
	return Client.Database("tracker").Collection(ChannelID)
}

func loadChannelTable(ChannelID string) *mongo.Collection {
	Table, ok := Tables[ChannelID]
	if !ok {
		Table = createChannelItemTableIfMissing(ChannelID)
	}
	return Table
}
