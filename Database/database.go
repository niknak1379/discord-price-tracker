package database

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	crawler "priceTracker/Crawler"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type TrackingInfo struct {
	URI       string 	`bson:"URI"`
	HtmlQuery string 	`bson:"HtmlQuery"`
}
type Price struct{
	Date time.Time      `bson:"Date"`
	Price int 			`bson:"Price"`
	Url string			`bson:"Url"`
}
type Item struct {
	Name         string `bson:"Name"`
	TrackingList []*TrackingInfo `bson:"TrackingList"`
	LowestPrice Price `bson:"LowestPrice"`
	PriceHistory []*Price `bson:"PriceHistory"`
	CurrentLowestPrice Price `bson:"CurrentLowestPrice"`
}
var Client *mongo.Client
var Table *mongo.Collection
var ctx context.Context
func AddItem(itemName string, uri string, query string) (Item, error){
	p, t, err := validateURI(uri, query)
	if err != nil{
		log.Print("invalid url", err)
		return Item{}, err
	}
	arr := []*TrackingInfo{&t}
	PriceArr := []*Price{&p}
	i := Item{
		Name: itemName,
		LowestPrice: p,					
		TrackingList: arr,
		PriceHistory: PriceArr,	
		CurrentLowestPrice: p,		
	}
	result, err := Table.InsertOne(ctx, i)
	if err != nil{
		log.Print(err)
	}
	log.Println("added new item with mongodb logs:", result)
	log.Println("lowest price being passed on", i.LowestPrice.Price, i.LowestPrice.Url)
	return i, err
}
func AddNewPrice(Name string, uri string, newPrice int, historicalLow int, date time.Time) (Price, error){
    
    price := Price{
        Price: newPrice,
        Url: uri,
        Date: date,
    }
    
    startOfDay := date.Truncate(24 * time.Hour)
    
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
    
    cursor, err := Table.Aggregate(context.TODO(), pipeline)
    if err != nil{
        return Price{}, err
    }
    defer cursor.Close(context.TODO())
    
    type Result struct {
        PriceHistory []*Price `bson:"PriceHistory"`
    }
    
    var results []Result
    if err = cursor.All(context.TODO(), &results); err != nil{
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
        UpdateLowestHistoricalPrice(Name, price)
    }
    
    filter := bson.M{"Name": Name}
    update := bson.M{"$push": bson.M{
        "PriceHistory": price,
    }} 
    
    var result Item
    opts := options.FindOneAndUpdate().SetProjection(bson.D{{"PriceHistory", 0}}).SetReturnDocument(options.After)
    err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
    if err != nil{
        log.Print("error in addingnewprice", err)
        return price, err
    }
    
    log.Printf("adding new price for %s with price %d for url %s", Name, newPrice, uri)
    return price, nil
}
func GetLowestHistoricalPrice(Name string) (Price, error){
	filter := bson.M{"Name": Name}
	opts := options.FindOne().SetProjection(bson.M{"LowestPrice": 1})
	var res Item
	err := Table.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil{
		return res.LowestPrice, err
	}
	log.Printf("getting lowest price of %d for %s", res.LowestPrice.Price, res.LowestPrice.Url)
	return res.LowestPrice, err
}
func UpdateLowestHistoricalPrice(Name string, newLow Price) (Item, error){
	filter := bson.M{"Name" : Name}
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{"PriceHisotry", 0}}).SetReturnDocument(options.After)
	update := bson.M {
		"$set" : bson.M{
			"LowestPrice": newLow,
		},
	}
	var res Item
	err := Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&res)
	if err != nil{
		log.Printf("error in updating lowest price", err)
		return res, err
	}
	log.Printf("updating lowest price of %d for %s", newLow.Price, Name)
	return res, err
}
func GetLowestPrice(Name string) (Price, error){
	filter := bson.M{"Name": Name}
	opts := options.FindOne().SetProjection(bson.M{"CurrentLowestPrice": 1})
	var res Item
	err := Table.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil{
		return res.LowestPrice, err
	}
	log.Printf("getting lowest current price of %d for %s", res.LowestPrice.Price, res.LowestPrice.Url)
	return res.LowestPrice, err
}
func UpdateLowestPrice(Name string, newLow Price) (Item, error){
	filter := bson.M{"Name" : bson.M{"$regex": "^" + Name + "$", "$options": "i"}}
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{"PriceHisotry", 0}}).SetReturnDocument(options.After)
	update := bson.M {
		"$set" : bson.M{
			"CurrentLowestPrice": newLow,
		},
	}
	var res Item
	err := Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&res)
	if err != nil{
		log.Print("error in updating lowest price", err)
		return res, err
	}
	log.Printf("updating lowest price of %d for %s", newLow.Price, Name)
	return res, err
}
func GetAllItems() []*Item {
	opts := options.Find().SetProjection(bson.D{{"PriceHistory", 0}})
	cursor, err := Table.Find(ctx, bson.M{}, opts)
	if err != nil{
		panic(err)
	}
	var result []*Item
	err = cursor.All(ctx, &result)
	defer cursor.Close(ctx)
	if err != nil{
		panic(err)
	}
	log.Println("getting all items")
	return result
}

func GetItem(itemName string) (Item, error) {
	var res Item
	filter := bson.M{"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"}}
	opts := options.FindOne().SetProjection(bson.D{{"PriceHistory", 0}})
	err := Table.FindOne(ctx, filter, opts).Decode(&res)
	if err != nil{
		return res, err
	}
	return res, err
}
// returns the price
func GetPriceHistory(Name string, date time.Time) ([]*Price, error){
	var res []*Price
	pipeline := mongo.Pipeline{
		bson.D{{"$match", bson.D{{"Name", Name}}}},
		bson.D{{"$unwind", bson.D{{"path", "$PriceHistory"}}}},
		bson.D{
			{"$unset",
				bson.A{
					"Name",
					"_id",
					"LowestPrice",
					"TrackingList",
				},
			},
		},
		bson.D{{"$sort", bson.D{{"PriceHistory.Date", 1}}}},
		bson.D{
			{"$project",
				bson.D{
					{"Date", "$PriceHistory.Date"},
					{"Price", "$PriceHistory.Price"},
					{"Url", "$PriceHistory.Url"},
				},
			},
		},
	}
	cursor, err := Table.Aggregate(ctx, pipeline)
	if err != nil{
		return res, err
	}
    if err = cursor.All(ctx, &res); err != nil {
        return res, err
    }
	defer cursor.Close(ctx)
	return res, err
}
func RemoveItem(itemName string)  *mongo.DeleteResult{
	filter := bson.M{"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"}}
	results, err := Table.DeleteOne(ctx, filter)
	if err != nil{
		panic(err)
	}
	return results
}

func AddTrackingInfo(itemName string, uri string, querySelector string) (Item, Price, error){
	p, t, err := validateURI(uri, querySelector)
	if err != nil{
		return Item{}, p, err
	}
	filter := bson.M{"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"}}
	
	update := bson.M{"$push": bson.M{
		"TrackingList": t,
		"PriceHistory": p,
	}} 
	var result Item
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{"PriceHistory", 0}}).SetReturnDocument(options.After)
	err = Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil{
		return result, p, err
	}
	return result, p, err
} 

func RemoveTrackingInfo(itemName string, uri string) (Item, error) {
	filter := bson.M{
		"Name": bson.M{"$regex": "^" + itemName + "$", "$options": "i"}, 
	}

	update := bson.M{"$pull": bson.M{"TrackingList": bson.M{"URI": uri}}}
	
	var result Item
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{"PriceHistory", 0}}).SetReturnDocument(options.After)
	err := Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil{
		return result, err
	}
	return result, err
}


func InitDB(ctx context.Context) {
	godotenv.Load()
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
	Table = Client.Database("tracker").Collection("Items")
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")
}
func validateURI(uri string, querySelector string) (Price, TrackingInfo, error){
	_, err := url.ParseRequestURI(uri)
	if err != nil{
		log.Print("invalid url", err)
		return Price{}, TrackingInfo{}, err
	}
	pr, err := crawler.GetPrice(uri, querySelector)
	log.Print("price of from validated int", pr)
	if err != nil{
		log.Print("invalid url", err)
		return Price{}, TrackingInfo{}, err
	}
	tracking := TrackingInfo{
		URI: uri,
		HtmlQuery: querySelector,
	}
	price := Price{
		Date: time.Now(),
		Price: pr,
		Url: uri,
	}
	return price, tracking, err
}

