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
	HtmlQuery string `bson:"htmlQuery"`
}
type Item struct {
	Name         string `bson:"Name"`
	TrackingList []*TrackingInfo `bson:"trackingList"`
}
var Client *mongo.Client
var Table *mongo.Collection
func AddItem(itemName string, uri string, query string, client *mongo.Client) *mongo.InsertOneResult{
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



func getAllItems() []Item {
	cursor, err := Table.Find(context.TODO(), bson.M{})
	if err == nil{
		panic(err)
	}
	var result []Item
	err = cursor.All(context.TODO(), result)
	if err == nil{
		panic(err)
	}
	return result
}

func getItem(itemName string) Item {
	var res Item
	err := Table.FindOne(context.TODO(), bson.M{}).Decode(res)
	if err == nil{
		panic(err)
	}
	return res
}
/*
func removeItem(itemName string) item {

}
func addTrackingInfo(itemName string, uri string) item {

}
func removeTrackingInfo(itemName string, uri string) item {

}
*/

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file")
	}
	// Use the SetServerAPIOptions() method to set the version of the Stable API on the client
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	opts := options.Client().ApplyURI(os.Getenv("MONGODB_URI")).SetServerAPIOptions(serverAPI)
	// Create a new client and connect to the server
	Client, _ = mongo.Connect(opts)
	if err != nil {
		panic(err)
	}
	
	// Send a ping to confirm a successful connection
	if err := Client.Ping(context.TODO(), readpref.Primary()); err != nil {
		panic(err)
	}
	Table := Client.Database("tracker").Collection("Items")
	fmt.Println("Pinged your deployment. You successfully connected to MongoDB!")
}
