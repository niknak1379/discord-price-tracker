package database

import (
	"log"
	"log/slog"
	"os"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

var (
	Tables []*mongo.Collection
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
}

