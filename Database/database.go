package database

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
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
}
type aggregateResult struct {
	Url string `bson:"_id"` //grouped by url so id is url string
	Prices []*Price `bson:"prices"`
}
var Client *mongo.Client
var Table *mongo.Collection
var ctx context.Context
func AddItem(itemName string, uri string, query string) *mongo.InsertOneResult{
	t := TrackingInfo{
		URI: uri,
		HtmlQuery: query,
	}
	arr := []*TrackingInfo{&t}
	PriceArr := []*Price{}
	p := Price{
		Date: time.Now(),
		Price: math.MaxInt32,
		Url: "",
	}
	i := Item{
		Name: itemName,
		LowestPrice: p,					// init 0 as default price
		TrackingList: arr,
		PriceHistory: PriceArr,			// init empty price arr
	}
	result, err := Table.InsertOne(ctx, i)
	if err != nil{
		panic(err)
	}
	log.Println("adding new item", itemName)
	return result
}

func AddNewPrice(Name string, uri string, newPrice int, oldPrice int, date time.Time) (Item, error){
	Price := Price{
		Price: newPrice,
		Url: uri,
		Date: date,
	}
	log.Printf("%d old price, %d new price bool: %b", oldPrice, newPrice, oldPrice > newPrice)
	if (newPrice < oldPrice) {
		UpdateLowestPrice(Name, Price)
	}
	filter := bson.M{"Name": Name}
	update := bson.M{"$push": bson.M{
		"PriceHistory": Price,
	}} 
	var result Item
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{"PriceHistory", 0}})
	err := Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil{
		return result, err
	}
	log.Printf("adding new price for %s with price %d for url %s", Name, newPrice, uri)
	return result, err
}
func GetLowestPrice(Name string) (Price, error){
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
func UpdateLowestPrice(Name string, newLow Price) (Item, error){
	filter := bson.M{"Name" : Name}
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{"PriceHisotry", 0}})
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
	filter := bson.M{"Name": itemName}
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
	filter := bson.M{"Name": itemName}
	results, err := Table.DeleteOne(ctx, filter)
	if err != nil{
		panic(err)
	}
	return results
}

func AddTrackingInfo(itemName string, uri string, querySelector string) (Item, error){
	filter := bson.M{"Name": itemName}
	t := TrackingInfo{
		URI:       uri,
		HtmlQuery: querySelector,
	}
	update := bson.M{"$push": bson.M{
		"TrackingList": t,
	}} 
	var result Item
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{"PriceHistory", 0}})
	err := Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil{
		return result, err
	}
	return result, err
} 

func RemoveTrackingInfo(itemName string, uri string) (Item, error) {
	filter := bson.M{
		"Name": itemName, 
	}

	update := bson.M{"$pull": bson.M{"TrackingList": bson.M{"URI": uri}}}
	
	var result Item
	opts := options.FindOneAndUpdate().SetProjection(bson.D{{"PriceHistory", 0}})
	err := Table.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)
	if err != nil{
		return result, err
	}
	return result, err
}


func InitDB(ctx context.Context, cancel context.CancelFunc) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}
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
	/* AddItem("hi", "hi", "htmlTag")
	fmt.Println("did add item")
	fmt.Println("get all items", GetAllItems())
	fmt.Println("get hi", GetItem("hi"))
	fmt.Println(AddTrackingInfo("hi", "second URI", "second Query"))
	fmt.Println(RemoveTrackingInfo("hi", "hi"))
	fmt.Println("get hi", GetItem("hi")) */
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")
}
