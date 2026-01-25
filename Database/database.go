package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"slices"
	"time"

	crawler "priceTracker/Crawler"
	types "priceTracker/Types"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type TrackingInfo struct {
	URI       string `bson:"URI"`
	HtmlQuery string `bson:"HtmlQuery"`
}
type Price struct {
	Date  time.Time `bson:"Date"`
	Price int       `bson:"Price"`
	Url   string    `bson:"Url"`
}
type AggregateReport struct {
	UniqueListings              int `bson:"UniqueListings"`
	AverageDaysUP               int `bson:"AverageDaysUP"`
	AveragePrice                int `bson:"AveragePrice"`
	PriceSTDEV                  int `bson:"PriceSTDEV"`
	AveragePriceWhenSold        int `bson:"AveragePriceWhenSold"`
	LowestPriceDuringTimePeriod int `bson:"LowestPriceDuringTimePeriod"`
}
type Item struct {
	Name                  string              `bson:"Name"`
	TrackingList          []TrackingInfo      `bson:"TrackingList"`
	LowestPrice           Price               `bson:"LowestPrice"`
	PriceHistory          []Price             `bson:"PriceHistory"`
	CurrentLowestPrice    Price               `bson:"CurrentLowestPrice"`
	Timer                 int                 `bson:"Timer"`
	Type                  string              `bson:"Type"`
	ImgURL                string              `bson:"ImgURL"`
	EbayListings          []types.EbayListing `bson:"EbayListings"`
	ListingsHistory       []types.EbayListing `bson:"ListingsHistory"`
	SevenDayAggregate     AggregateReport     `bson:"SevenDayAggregate"`
	SuppressNotifications bool                `bson:"SuppressNotifications"`
}

var (
	Client *mongo.Client
	Table  *mongo.Collection
	ctx    context.Context
)

func AddItem(itemName string, uri string, query string, Type string, Timer int, Channel *Channel) (Item, error) {
	Table, err := loadChannelTable(Channel.ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return Item{}, err
	}
	if Channel.TotalItems >= 25 {
		return Item{}, errors.New("Channel Capacity Reached, add to a separate channel")
	}
	updateChannelLength(Channel.ChannelID, 1)
	if Timer >= 0 {
		return Item{}, errors.New("Invalid Timer value")
	}
	p, t, err := validateURI(uri, query)
	if err != nil {
		slog.Error("invalid url for add", slog.Any("Error", err))
		return Item{}, err
	}
	imgURL := crawler.GetOpenGraphPic(uri)
	ebayListings, _ := crawler.GetSecondHandListings(itemName, p.Price,
		Channel.Lat, Channel.Long, Channel.Distance, Type, Channel.LocationCode)
	slices.SortFunc(ebayListings, func(a, b types.EbayListing) int {
		return b.Price - a.Price
	})
	arr := []TrackingInfo{t}
	PriceArr := []Price{p}
	i := Item{
		Name:                  itemName,
		ImgURL:                imgURL,
		LowestPrice:           p,
		Type:                  Type,
		TrackingList:          arr,
		PriceHistory:          PriceArr,
		CurrentLowestPrice:    p,
		Timer:                 Timer,
		EbayListings:          ebayListings,
		ListingsHistory:       ebayListings,
		SuppressNotifications: false,
	}
	_, err = Table.InsertOne(ctx, i)
	if err != nil {
		slog.Error("Error", slog.Any("Error", err))
	}
	UpdateAggregateReport(itemName, Channel.ChannelID)
	i, err = GetItem(itemName, Channel.ChannelID)
	return i, err
}

func EditTimer(Name string, NewTimer int, ChannelID string) error {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("Could not load channel from db", slog.Any("Error", err))
		return err
	}
	if NewTimer >= 0 {
		return errors.New("Invalid Timer value")
	}
	update := bson.M{
		"$set": bson.M{
			"Timer": NewTimer,
		},
	}
	res := Table.FindOneAndUpdate(ctx, bson.M{"Name": Name}, update)
	return res.Err()
}

func EditSuppress(Name string, Suppress bool, ChannelID string) error {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("Could not load channel from db", slog.Any("Error", err))
		return err
	}
	update := bson.M{
		"$set": bson.M{
			"SuppressNotifications": Suppress,
		},
	}
	res := Table.FindOneAndUpdate(ctx, bson.M{"Name": Name}, update)
	return res.Err()
}

func EditName(oldName string, newName string, ChannelID string) (Item, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("Could not load channel from db", slog.Any("Error", err))
		return Item{}, err
	}
	var res Item
	filter := bson.M{"Name": oldName}
	update := bson.M{"$set": bson.M{"Name": newName}}
	opts := options.FindOneAndUpdate().SetProjection(bson.D{
		{Key: "PriceHistory", Value: 0},
		{Key: "ListingsHistory", Value: 0},
	}).SetReturnDocument(options.After)
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&res)
	if err != nil {
		slog.Error("failed to change name of title",
			slog.String("Name", oldName),
			slog.Any("value", err),
		)
		return Item{}, err
	}
	return res, err
}

// method itself checks if the price is a duplicate and if so does not add it
func AddNewPrice(Name string, uri string, newPrice int, historicalLow int, date time.Time, ChannelID string) (Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return Price{}, err
	}
	price := Price{
		Price: newPrice,
		Url:   uri,
		Date:  date,
	}

	startOfDay := date.Truncate(24 * time.Hour)

	// pipeline to see if price is duplicate
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"Name": Name}}},
		bson.D{{Key: "$project", Value: bson.M{
			"PriceHistory": bson.M{
				"$filter": bson.M{
					"input": "$PriceHistory",
					"as":    "price",
					"cond": bson.M{
						"$and": []bson.M{
							{"$gte": []interface{}{"$$price.Date", startOfDay}},
							{"$eq": []interface{}{"$$price.Url", uri}},
						},
					},
				},
			},
		}}},
	}

	cursor, err := Table.Aggregate(ctx, pipeline)
	if err != nil {
		return Price{}, err
	}
	defer cursor.Close(ctx)

	type Result struct {
		PriceHistory []*Price `bson:"PriceHistory"`
	}

	var results []Result
	if err = cursor.All(ctx, &results); err != nil {
		return Price{}, err
	}

	// Check if price unchanged today
	if len(results) > 0 && len(results[0].PriceHistory) > 0 {
		for _, p := range results[0].PriceHistory {
			if p.Price == newPrice {
				slog.Info("Price Same, Skipping todays update")
				return price, nil
			}
		}
	}

	if newPrice < historicalLow {
		UpdateLowestHistoricalPrice(Name, price, ChannelID)
	}

	filter := bson.M{"Name": Name}
	update := bson.M{"$push": bson.M{
		"PriceHistory": price,
	}}

	var result Item
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{Key: "PriceHistory", Value: 0}}).SetReturnDocument(options.After)
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		slog.Error("couldnt add new price", slog.Any("Error", err))
		return price, err
	}
	return price, nil
}

func GetLowestHistoricalPrice(Name string, ChannelID string) (Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return Price{}, err
	}
	filter := bson.M{"Name": Name}
	opts := options.FindOne().SetProjection(bson.M{"LowestPrice": 1})
	var res Item
	err = Table.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil {
		return res.LowestPrice, err
	}
	return res.LowestPrice, err
}

func UpdateLowestHistoricalPrice(Name string, newLow Price, ChannelID string) (Item, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return Item{}, err
	}
	filter := bson.M{"Name": Name}
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{Key: "PriceHisotry", Value: 0}}).SetReturnDocument(options.After)
	update := bson.M{
		"$set": bson.M{
			"LowestPrice": newLow,
		},
	}
	var res Item
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&res)
	if err != nil {
		slog.Error("error updating DB", slog.Any("Error", err))
		return res, err
	}
	return res, err
}

func GetLowestPrice(Name string, ChannelID string) (Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return Price{}, err
	}
	filter := bson.M{"Name": Name}
	opts := options.FindOne().SetProjection(bson.M{"CurrentLowestPrice": 1})
	var res Item
	err = Table.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil {
		return res.LowestPrice, err
	}

	return res.LowestPrice, err
}

func UpdateLowestPrice(Name string, newLow Price, ChannelID string) (Item, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return Item{}, err
	}
	filter := bson.M{"Name": bson.M{"$regex": "^" + Name + "$", "$options": "i"}}
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{Key: "PriceHisotry", Value: 0}}).SetReturnDocument(options.After)
	update := bson.M{
		"$set": bson.M{
			"CurrentLowestPrice": newLow,
		},
	}
	var res Item
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&res)
	if err != nil {
		slog.Error("could not update lowest price", slog.Any("Error", err))
		return res, err
	}
	return res, err
}

func GetAllItems(ChannelID string) []*Item {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return []*Item{}
	}
	opts := options.Find().SetProjection(bson.D{{Key: "PriceHistory", Value: 0}})
	cursor, err := Table.Find(ctx, bson.M{}, opts)
	if err != nil {
		panic(err)
	}
	var result []*Item
	err = cursor.All(ctx, &result)
	defer cursor.Close(ctx)
	if err != nil {
		panic(err)
	}
	return result
}

func GetEbayListings(itemName string, ChannelID string) ([]types.EbayListing, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return []types.EbayListing{}, err
	}
	var res Item
	filter := bson.M{"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"}}
	opts := options.FindOne().SetProjection(bson.D{{Key: "EbayListings", Value: 1}})
	err = Table.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil {
		return res.EbayListings, err
	}
	return res.EbayListings, err
}

func UpdateEbayListings(itemName string, listingsArr []types.EbayListing, ChannelID string) error {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return err
	}
	filter := bson.M{"Name": itemName}
	slices.SortFunc(listingsArr, func(a, b types.EbayListing) int {
		return b.Price - a.Price
	})
	startOfDay := time.Now().Truncate(24 * time.Hour)
	var filteredListigArr []types.EbayListing // filtered array
	// pipeline to see if price is duplicate
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "Name", Value: itemName}}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "ListingsHistory", Value: bson.M{
				"$filter": bson.M{
					"input": "$ListingsHistory",
					"as":    "listing",
					"cond":  bson.M{"$gte": bson.A{"$$listing.Date", startOfDay}},
				},
			}},
		}}},
	}
	cursor, err := Table.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	type Result struct {
		ListingHistory []types.EbayListing `bson:"ListingsHistory"`
	}

	var results []Result
	if err = cursor.All(ctx, &results); err != nil {
		return err
	}
	listingMap := make(map[string]types.EbayListing) // maps url to item
	if len(results) != 0 {
		for _, Listing := range results[0].ListingHistory {
			listingMap[Listing.URL] = Listing
		}
	}
	for _, Listing := range listingsArr {
		if oldListing, ok := listingMap[Listing.URL]; ok {
			if oldListing.Price == Listing.Price {
				continue
			} else {
				filteredListigArr = append(filteredListigArr, Listing)
			}
		} else {
			filteredListigArr = append(filteredListigArr, Listing)
		}
	}
	var update bson.M

	slog.Info("listingHistory objects", slog.Any("returned Array", listingsArr),
		slog.Any("filtered array", filteredListigArr))
	if len(filteredListigArr) != 0 {
		update = bson.M{
			"$set": bson.M{
				"EbayListings": listingsArr,
			},
			"$push": bson.M{
				"ListingsHistory": bson.M{
					"$each": filteredListigArr,
				},
			},
		}
	} else {
		update = bson.M{
			"$set": bson.M{
				"EbayListings": listingsArr,
			},
		}
	}

	var result Item
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{Key: "PriceHistory", Value: 0}})
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)

	return err
}

func GetItem(itemName string, ChannelID string) (Item, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return Item{}, err
	}
	var res Item
	filter := bson.M{"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"}}
	opts := options.FindOne().SetProjection(bson.D{{Key: "PriceHistory", Value: 0}})
	err = Table.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil {
		return res, err
	}
	return res, err
}

func RemoveItem(itemName string, ChannelID string) int64 {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return 0
	}
	filter := bson.M{"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"}}
	results, err := Table.DeleteOne(ctx, filter)
	if err != nil {
		slog.Error("couldnt remove from DB", slog.Any("Error", err))
	}
	updateChannelLength(ChannelID, -1)
	return results.DeletedCount
}

func AddTrackingInfo(itemName string, uri string, querySelector string, ChannelID string) (Item, Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return Item{}, Price{}, err
	}
	p, t, err := validateURI(uri, querySelector)
	if err != nil {
		return Item{}, p, err
	}
	filter := bson.M{"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"}}

	update := bson.M{"$push": bson.M{
		"TrackingList": t,
		"PriceHistory": p,
	}}
	var result Item
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{Key: "PriceHistory", Value: 0}}).SetReturnDocument(options.After)
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil {
		return result, p, err
	}
	return result, p, err
}

func RemoveTrackingInfo(itemName string, index int, ChannelID string) (Item, error) {
	var result Item
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		slog.Error("couldnt load channel", slog.Any("Error", err))
		return Item{}, err
	}
	filter := bson.M{
		"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"},
	}

	update1 := bson.M{
		"$unset": bson.M{
			fmt.Sprintf("TrackingList.%d", index): "",
		},
	}
	_, err = Table.UpdateOne(ctx, filter, update1)
	if err != nil {
		return result, err
	}

	update2 := bson.M{
		"$pull": bson.M{
			"TrackingList": nil,
		},
	}
	opts := options.FindOneAndUpdate().
		SetProjection(bson.D{{Key: "PriceHistory", Value: 0}}).
		SetReturnDocument(options.After)
	err = Table.FindOneAndUpdate(ctx, filter, update2, opts).Decode(&result)
	if err != nil {
		return result, err
	}
	return result, err
}

func InitDB(context context.Context) {
	godotenv.Load()
	ctx = context
	var err error
	// Use the SetServerAPIOptions() method to set the version of the Stable API on the client
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(os.Getenv("MONGODB_URI")).SetServerAPIOptions(serverAPI)
	// Create a new client and connect to the server
	Client, err = mongo.Connect(opts)
	if err != nil {
		panic(err)
	}

	// Send a ping to confirm a successful connection
	if err := Client.Ping(ctx, readpref.Primary()); err != nil {
		panic(err)
	}
	loadDBTables()
	slog.Info("DB Successfully Pinged")
}

func validateURI(uri string, querySelector string) (Price, TrackingInfo, error) {
	_, err := url.ParseRequestURI(uri)
	if err != nil {
		slog.Error("Invalid url")
		return Price{}, TrackingInfo{}, err
	}
	pr, err := crawler.GetPrice(uri, querySelector, true)
	if err != nil {
		return Price{}, TrackingInfo{}, err
	}
	tracking := TrackingInfo{
		URI:       uri,
		HtmlQuery: querySelector,
	}
	price := Price{
		Date:  time.Now(),
		Price: pr,
		Url:   uri,
	}
	return price, tracking, err
}
