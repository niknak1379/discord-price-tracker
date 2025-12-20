package scheduler

import (
	"context"
	"log"
	"math/rand/v2"
	crawler "priceTracker/Crawler"
	database "priceTracker/Database"

	discord "priceTracker/Discord"
	"time"
)
func InitScheduler(ctx context.Context, cancel context.CancelFunc){
	// -------------------- set timer for daily scrapping -------------//
	updateAllPrices()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		log.Println("setting ticker in crawler")
		defer ticker.Stop()
		for {
			select{
			case <- ctx.Done():
				return
			case <- ticker.C:
				updateAllPrices()
			}
		}
	}()
	defer cancel()
}
func updateAllPrices(){
	itemsArr := database.GetAllItems()
	for _,v := range itemsArr{
		date := time.Now()
		oldLow, err := database.GetLowestPrice(v.Name)
		if err != nil{
			log.Print(err)
			continue
		}
		for _,t := range v.TrackingList{
			r := rand.IntN(10)
			timer := time.NewTimer(time.Duration(r) * time.Second)
			<- timer.C
			updatePrice(v.Name, t.URI, t.HtmlQuery, oldLow.Price, date)
		}
	}
}
func updatePrice(Name string, URI string, HtmlQuery string, oldLow int, date time.Time){
	newPrice, err := crawler.GetPrice(URI, HtmlQuery)
	if err != nil {
		log.Print("error getting price in updatePrice", err)
		discord.CrawlErrorAlert(discord.Discord, Name, URI, err)
		return
	}
	database.AddNewPrice(Name, URI, newPrice, oldLow, date)
	if oldLow > newPrice {
			discord.LowestPriceAlert(discord.Discord, Name, newPrice, oldLow, URI)
	}
}