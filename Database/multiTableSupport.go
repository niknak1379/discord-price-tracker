package database

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"sync"

	crawler "priceTracker/Crawler"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Channel object with its data
type Channel struct {
	ChannelID    string  `bson:"ChannelID"`
	Lat          float64 `bson:"Lat"`
	Long         float64 `bson:"Long"`
	Distance     int     `bson:"Distance"`
	LocationCode string  `bson:"LocationCode"`
	TotalItems   int     `bson:"TotalItems"`
}

// Guild represents a Discord server where the bot is installed.
type Guild struct {
	GuildID         string `bson:"GuildID"`
	SystemChannelID string `bson:"SystemChannelID"`
}

var (
	// has the mongo table stored
	Tables = make(map[string]*mongo.Collection)
	// has the distance, lat, long, and other facebook info stored
	ChannelMap  = make(map[string]*Channel)
	ChannelLock sync.Mutex
	// map of joined guilds(servers)
	GuildMap = make(map[string]*Guild)
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
	slog.Info("channels", slog.Any("IDs:", ChannelsArr))
	ChannelLock.Lock()
	for _, IDString := range ChannelsArr {
		table := Client.Database("tracker").Collection(IDString.ChannelID)
		Tables[IDString.ChannelID] = table
		ChannelMap[IDString.ChannelID] = &Channel{
			ChannelID:    IDString.ChannelID,
			Lat:          IDString.Lat,
			Long:         IDString.Long,
			Distance:     IDString.Distance,
			LocationCode: IDString.LocationCode,
			TotalItems:   IDString.TotalItems,
		}
		if IDString.Lat == 0 || IDString.Long == 0 || IDString.Distance == 0 {
			log.Panic("Could not load Channel, lat, long or distance empty")
		}
	}
	loadGuilds()
	ChannelLock.Unlock()
}

// loadGuilds loads all tracked guilds from the database into memory.
func loadGuilds() {
	guildTable := Client.Database("tracker").Collection("Guilds")
	cursor, err := guildTable.Find(ctx, bson.M{})
	if err != nil {
		slog.Error("could not load guilds", slog.Any("Error", err))
		return
	}
	defer cursor.Close(ctx)

	var guilds []Guild
	if err := cursor.All(ctx, &guilds); err != nil {
		slog.Error("could not read guild results", slog.Any("Error", err))
		return
	}

	for _, guild := range guilds {
		GuildMap[guild.GuildID] = &guild
	}
	slog.Info("guilds loaded", slog.Int("count", len(guilds)))
}

// IsFirstTimeJoin checks if this is the first time the bot joined a guild.
// If so, adds the guild to the database and returns true.
// Returns false if the guild already exists.
// Parameters
//   - guildID: ID of the guild the bot was added to
//   - systemChannelID: the default channel automated messages are sent
func IsFirstTimeJoin(guildID, systemChannelID string) bool {
	ChannelLock.Lock()
	if _, exists := GuildMap[guildID]; exists {
		return false
	}

	// Add guild to database and memory
	guild := &Guild{
		GuildID:         guildID,
		SystemChannelID: systemChannelID,
	}

	guildTable := Client.Database("tracker").Collection("Guilds")
	_, err := guildTable.InsertOne(ctx, guild)
	if err != nil {
		slog.Error("could not insert guild", slog.Any("Error", err))
		return false
	}

	GuildMap[guildID] = guild
	ChannelLock.Unlock()
	return true
}

// GetChannelInfo retrieves the channel configuration for a given channel ID.
//
// Parameters:
//   - ChannelID: the Discord channel ID
//
// Returns the channel configuration or nil if not found.
func GetChannelInfo(ChannelID string) (*Channel, bool) {
	slog.Info("Getting Channel Info", slog.String("Channel ID", ChannelID))
	ChannelLock.Lock()
	Channel, ok := ChannelMap[ChannelID]
	ChannelLock.Unlock()
	return Channel, ok
}

// UpdateChannelOrCreateChannelItemTableIfMissing creates or updates a channel configuration.
// It geocodes the location and stores the channel settings in the database.
//
// Parameters:
//   - ChannelID: the Discord channel ID
//   - Location: the location string (e.g., "Los Angeles, CA")
//   - LocationCode: the Facebook Marketplace location code
//   - maxDistance: the maximum search distance in miles
//
// Returns any error encountered.
func UpdateChannelOrCreateChannelItemTableIfMissing(ChannelID string, Location string, LocationCode string, maxDistance int) error {
	slog.Info("setup Channel Called", slog.String("ChannelID", ChannelID))
	Lat, Long, err := crawler.GetCoordinates(Location)
	if err != nil {
		return err
	}
	Channel := Channel{
		ChannelID:    ChannelID,
		Lat:          Lat,
		Long:         Long,
		Distance:     maxDistance,
		LocationCode: LocationCode,
		TotalItems:   0,
	}
	ChannelLock.Lock()
	// if channelID already exists, just update the Coordinates in DB and memory
	if _, ok := Tables[ChannelID]; ok {
		slog.Info("Channel Already Exists Updating")
		ChannelMap[ChannelID] = &Channel
		update := bson.M{
			"$set": bson.M{
				"Distance":     maxDistance,
				"Lat":          Lat,
				"Long":         Long,
				"LocationCode": LocationCode,
			},
		}
		ChannelMap[ChannelID].Distance = maxDistance
		ChannelMap[ChannelID].Lat = Lat
		ChannelMap[ChannelID].Long = Long
		ChannelMap[ChannelID].LocationCode = LocationCode

		ChannelTable := Client.Database("tracker").Collection("ChannelIDs")
		ChannelTable.FindOneAndUpdate(ctx, bson.M{"ChannelID": ChannelID}, update)
		return nil
	}
	slog.Info("New Channel, creating in DB")
	err = Client.Database("tracker").CreateCollection(context.TODO(), ChannelID)
	if err != nil {
		return err
	}
	// --------------- call to get coordinates goes here --------

	Client.Database("tracker").Collection("ChannelIDs").InsertOne(ctx, Channel)

	// this portion is deprecated since im moving off atlas
	// Table := Client.Database("tracker").Collection(ChannelID)
	// opts := options.SearchIndexes().SetName(ChannelID).SetType("search")
	// searchIndexModel := mongo.SearchIndexModel{
	// 	Definition: bson.D{
	// 		{Key: "mappings", Value: bson.D{
	// 			{Key: "dynamic", Value: false},
	// 			{Key: "fields", Value: bson.D{
	// 				{Key: "Name", Value: bson.D{
	// 					{Key: "type", Value: "autocomplete"},
	// 				}},
	// 			}},
	// 		}},
	// 	},
	// 	Options: opts,
	// }
	// // Creates the index
	// _, err = Table.SearchIndexes().CreateOne(ctx, searchIndexModel)
	// if err != nil {
	// 	return err
	// }
	table := Client.Database("tracker").Collection(ChannelID)
	Tables[ChannelID] = table
	ChannelMap[ChannelID] = &Channel
	ChannelLock.Unlock()

	return err
}

// ChannelDeleteHandler removes a channel from the database and memory.
//
// Parameters:
//   - ChannelID: the Discord channel ID to delete
func ChannelDeleteHandler(ChannelID string) {
	ChannelLock.Lock()
	if _, ok := Tables[ChannelID]; ok {
		ChannelTable := Client.Database("tracker").Collection("ChannelIDs")
		ItemArr := GetAllItems(ChannelID, ExludedFields)
		for _, Item := range ItemArr {
			sendItemChangeEvent(Item, Remove, *ChannelMap[ChannelID])
		}
		ChannelTable.FindOneAndDelete(ctx, bson.M{"ChannelID": ChannelID})
		delete(Tables, ChannelID)
		delete(ChannelMap, ChannelID)
	}
	ChannelLock.Unlock()
}

func loadChannelTable(ChannelID string) (*mongo.Collection, error) {
	Table, ok := Tables[ChannelID]
	if !ok {
		slog.Error("failed load Channel, channel has to be setup",
			slog.String("ChannelID", ChannelID),
		)
		//<------ make this a specific error that propogates
		// that forces the crawl thing to send error?
		err := errors.New("channel not found in db, call setup function first")
		return Table, err
	}
	return Table, nil
}

func getChannelLength(ChannelID string) (int, error) {
	Channel, ok := ChannelMap[ChannelID]
	if !ok {
		return 0, errors.New("Channel Not found")
	}
	return Channel.TotalItems, nil
}

func updateChannelLength(ChannelID string, Diff int) error {
	ChannelLock.Lock()
	Len, err := getChannelLength(ChannelID)
	if err != nil {
		return err
	} else if Diff+Len < 0 {
		slog.Error("Illegal Channel Length",
			slog.String("ChannelID", ChannelID),
			slog.Int("Diff", Diff),
			slog.Int("Length", Len),
		)
		return errors.New("Illegal Channel Length")
	}
	slog.Info("Updating Channel Length",
		slog.String("ChannelID", ChannelID),
		slog.Int("Diff", Diff),
		slog.Int("Length", Len),
	)
	update := bson.M{
		"$set": bson.M{
			"TotalItems": Diff + Len,
		},
	}

	Table := Client.Database("tracker").Collection("ChannelIDs")
	res := Table.FindOneAndUpdate(ctx, bson.M{"ChannelID": ChannelID}, update)
	ChannelMap[ChannelID].TotalItems = Diff + Len
	ChannelLock.Unlock()
	return res.Err()
}
