package database

import (
	"context"
	"fmt"
	"log"
	"os"

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
type Item struct {
	Name         string `bson:"Name"`
	TrackingList []*TrackingInfo `bson:"TrackingList"`
}
var Client *mongo.Client
var Table *mongo.Collection
func AddItem(itemName string, uri string, query string) *mongo.InsertOneResult{
	t := TrackingInfo{
		URI: uri,
		HtmlQuery: query,
	}
	arr := []*TrackingInfo{&t}
	i := Item{
		Name: itemName,
		TrackingList: arr,
	}
	fmt.Println("table", Table)
	result, err := Table.InsertOne(context.TODO(), i)
	if err != nil{
		panic(err)
	}
	return result
}



func GetAllItems() []Item {
	cursor, err := Table.Find(context.TODO(), bson.M{})
	if err != nil{
		panic(err)
	}
	var result []Item
	err = cursor.All(context.TODO(), &result)
	defer cursor.Close(context.TODO())
	if err != nil{
		panic(err)
	}
	return result
}

func GetItem(itemName string) Item {
	var res Item
	filter := bson.M{"Name": itemName}
	err := Table.FindOne(context.TODO(), filter).Decode(&res)
	if err != nil{
		panic(err)
	}
	return res
}

func RemoveItem(itemName string)  *mongo.DeleteResult{
	filter := bson.M{"Name": itemName}
	results, err := Table.DeleteOne(context.TODO(), filter)
	if err != nil{
		panic(err)
	}
	return results
}

func AddTrackingInfo(itemName string, uri string, querySelector string) Item{
	filter := bson.M{"Name": itemName}
	t := TrackingInfo{
		URI:       uri,
		HtmlQuery: querySelector,
	}
	update := bson.M{"$push": bson.M{
		"TrackingList": t,
	}} 
	var result Item
	err := Table.FindOneAndUpdate(context.TODO(), filter, update).Decode(&result)
	if err != nil{
		panic(err)
	}
	return result
} 

func RemoveTrackingInfo(itemName string, uri string) Item {
	filter := bson.M{
		"Name": itemName, 
	}

	update := bson.M{"$pull": bson.M{"TrackingList": bson.M{"URI": uri}}}
	
	var result Item
	err := Table.FindOneAndUpdate(context.TODO(), filter, update).Decode(&result)
	if err != nil{
		panic(err)
	}
	return result
}


func init() {
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
	if err := Client.Ping(context.TODO(), readpref.Primary()); err != nil {
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
