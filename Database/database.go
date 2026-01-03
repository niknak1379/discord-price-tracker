package database

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	crawler "priceTracker/Crawler"
	types "priceTracker/Types"
	"slices"
	"time"

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

type Item struct {
	Name               string              `bson:"Name"`
	TrackingList       []TrackingInfo      `bson:"TrackingList"`
	LowestPrice        Price               `bson:"LowestPrice"`
	PriceHistory       []Price             `bson:"PriceHistory"`
	CurrentLowestPrice Price               `bson:"CurrentLowestPrice"`
	Type               string              `bson:"Type"`
	ImgURL             string              `bson:"ImgURL"`
	EbayListings       []types.EbayListing `bson:"EbayListings"`
	ListingsHistory    []types.EbayListing `bson:"ListingsHistory"`
}

var (
	Client *mongo.Client
	Table  *mongo.Collection
	ctx    context.Context
)

func AddItem(itemName string, uri string, query string, Type string, Channel Channel) (Item, error) {
	Table, err := loadChannelTable(Channel.ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
		return Item{}, err
	}
	p, t, err := validateURI(uri, query)
	if err != nil {
		log.Print("invalid url", err)
		return Item{}, err
	}
	imgURL := crawler.GetOpenGraphPic(uri)
	ebayListings, _ := crawler.GetSecondHandListings(itemName, p.Price,
		Channel.Lat, Channel.Long, Channel.Distance, Type)
	slices.SortFunc(ebayListings, func(a, b types.EbayListing) int {
		return b.Price - a.Price
	})
	arr := []TrackingInfo{t}
	PriceArr := []Price{p}
	i := Item{
		Name:               itemName,
		ImgURL:             imgURL,
		LowestPrice:        p,
		Type:               Type,
		TrackingList:       arr,
		PriceHistory:       PriceArr,
		CurrentLowestPrice: p,
		EbayListings:       ebayListings,
		ListingsHistory:    ebayListings,
	}
	result, err := Table.InsertOne(ctx, i)
	if err != nil {
		log.Print(err)
	}
	log.Println("added new item with mongodb logs:", result)
	log.Println("lowest price being passed on", i.LowestPrice.Price, i.LowestPrice.Url)
	return i, err
}

func EditName(oldName string, newName string, ChannelID string) (Item, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
		return Item{}, err
	}
	var res Item
	filter := bson.M{"Name": oldName}
	update := bson.M{"$set": bson.M{"Name": newName}}
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{Key: "PriceHistory", Value: 0}}).SetReturnDocument(options.After)
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&res)
	if err != nil {
		fmt.Println("error changing name of item, ", oldName)
		return Item{}, err
	}
	return res, err
}

// method itself checks if the price is a duplicate and if so does not add it
func AddNewPrice(Name string, uri string, newPrice int, historicalLow int, date time.Time, ChannelID string) (Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
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
				log.Println("price for todays crawl has not changed, skipping db update")
				return price, nil
			}
		}
	}

	// Check for lowest historical price
	log.Printf("%d old price, %d new price", historicalLow, newPrice)
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
		log.Print("error in addingnewprice", err)
		return price, err
	}

	log.Printf("adding new price for %s with price %d for url %s", Name, newPrice, uri)
	return price, nil
}

func GetLowestHistoricalPrice(Name string, ChannelID string) (Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
		return Price{}, err
	}
	filter := bson.M{"Name": Name}
	opts := options.FindOne().SetProjection(bson.M{"LowestPrice": 1})
	var res Item
	err = Table.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil {
		return res.LowestPrice, err
	}
	log.Printf("getting lowest price of %d for %s", res.LowestPrice.Price, res.LowestPrice.Url)
	return res.LowestPrice, err
}

func UpdateLowestHistoricalPrice(Name string, newLow Price, ChannelID string) (Item, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
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
		log.Print("error in updating lowest price", err)
		return res, err
	}
	log.Printf("updating lowest price of %d for %s", newLow.Price, Name)
	return res, err
}

func GetLowestPrice(Name string, ChannelID string) (Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
		return Price{}, err
	}
	filter := bson.M{"Name": Name}
	opts := options.FindOne().SetProjection(bson.M{"CurrentLowestPrice": 1})
	var res Item
	err = Table.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil {
		return res.LowestPrice, err
	}
	log.Printf("getting lowest current price of %d for %s", res.LowestPrice.Price, res.LowestPrice.Url)
	return res.LowestPrice, err
}

func UpdateLowestPrice(Name string, newLow Price, ChannelID string) (Item, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
		return Item{}, err
	}
	filter := bson.M{"Name": bson.M{"$regex": "^" + Name + "$", "$options": "i"}}
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{Key: "PriceHisotry", Value: 0}}).SetReturnDocument(options.After)
	update := bson.M{
		"$set": bson.M{
			"CurrentLowstPrice": newLow,
		},
	}
	var res Item
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&res)
	if err != nil {
		log.Print("error in updating lowest price", err)
		return res, err
	}
	log.Printf("updating lowest price of %d for %s", newLow.Price, Name)
	return res, err
}

func GetAllItems(ChannelID string) []*Item {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
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
	log.Println("getting all items", result)
	return result
}

func GetEbayListings(itemName string, ChannelID string) ([]types.EbayListing, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
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
		log.Print("Could not load Channel from DB")
		return err
	}
	filter := bson.M{"Name": itemName}
	slices.SortFunc(listingsArr, func(a, b types.EbayListing) int {
		return b.Price - a.Price
	})
	var update bson.M
	if len(listingsArr) == 0{
		update = bson.M{
			"$set": bson.M{
				"EbayListings": listingsArr,
			},
		}
	}else{
		update = bson.M{
			"$set": bson.M{
				"EbayListings": listingsArr,
			},
			"$push": bson.M{
				"ListingsHistory": bson.M{
					"$each": listingsArr,
				},
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
		log.Print("Could not load Channel from DB")
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

// returns the price
func GetPriceHistory(Name string, date time.Time, ChannelID string) ([]*Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
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
		}}}, // Fixed: Added proper closing braces
		bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$ListingsHistory"}}}},
		bson.D{
			{
				Key: "$unset",
				Value: bson.A{
					"Name",
					"_id",
					"LowestPrice",
					"TrackingList",
					"ListingsHistory",
				},
			},
		},
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
		return newRes, err
	}
	if err = cursor.All(ctx, &newRes); err != nil {
		return newRes, err
	}

	// ------------ pipeline for getting used Price -------------
	usedRes := []*Price{}
	usedPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "Name", Value: bson.D{
				{Key: "$regex", Value: "^" + Name + "$"},
				{Key: "$options", Value: "i"},
			}},
		}}}, // Fixed: Added proper closing braces
		bson.D{{Key: "$unwind", Value: bson.D{{Key: "path", Value: "$Listing"}}}},
		bson.D{
			{
				Key: "$unset",
				Value: bson.A{
					"Name",
					"_id",
					"LowestPrice",
					"TrackingList",
					"PriceHistory",
				},
			},
		},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "ListingsHistory.Date", Value: 1}}}},
		bson.D{
			{
				Key: "$project",
				Value: bson.D{
					{Key: "Date", Value: "$PriceHistory.Date"},
					{Key: "Price", Value: "$PriceHistory.Price"},
					{Key: "Url", Value: "USED"},
					// add average used and lowest used as separate data points
				},
			},
		},
	}
	cursor, err = Table.Aggregate(ctx, usedPipeline)
	if err != nil {
		return newRes, err
	}
	if err = cursor.All(ctx, &usedRes); err != nil {
		return usedRes, err
	}
	defer cursor.Close(ctx)

	return append(newRes, usedRes...), err
}

func RemoveItem(itemName string, ChannelID string) int64 {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
		return 0
	}
	filter := bson.M{"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"}}
	results, err := Table.DeleteOne(ctx, filter)
	if err != nil {
		log.Print("err in remove item", err)
	}
	return results.DeletedCount
}

func AddTrackingInfo(itemName string, uri string, querySelector string, ChannelID string) (Item, Price, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
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

func RemoveTrackingInfo(itemName string, uri string, ChannelID string) (Item, error) {
	Table, err := loadChannelTable(ChannelID)
	if err != nil {
		log.Print("Could not load Channel from DB")
		return Item{}, err
	}
	filter := bson.M{
		"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"},
	}

	update := bson.M{"$pull": bson.M{"TrackingList": bson.M{"URI": uri}}}

	var result Item
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{Key: "PriceHistory", Value: 0}}).SetReturnDocument(options.After)
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
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
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")
}

func validateURI(uri string, querySelector string) (Price, TrackingInfo, error) {
	_, err := url.ParseRequestURI(uri)
	if err != nil {
		log.Print("invalid url", err)
		return Price{}, TrackingInfo{}, err
	}
	pr, err := crawler.GetPrice(uri, querySelector)
	log.Print("price of from validated int", pr)
	if err != nil {
		log.Print("invalid url", err)
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
