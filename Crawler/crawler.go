package crawler

import (
	"log"
	database "priceTracker/Database"
	"priceTracker/bot"
	"time"
)
func init(){
	itemsArr := database.GetAllItems()
	for _,v := range itemsArr{
		date := time.Now()
		oldLow, err := database.GetLowestPrice(v.Name)
		if err != nil{
			log.Print(err)
			continue
		}
		for _,t := range v.TrackingList{
			newPrice, err := getPrice(t.URI, t.HtmlQuery)
			if err != nil {
				log.Print(err)
				bot.CrawlErrorAlert(bot.Discord, v.Name, t.URI, err)
				continue
			}
			database.AddNewPrice(v.Name, t.URI, newPrice, oldLow.Price, date)
		}
	}
}

func getPrice(uri string, querySelector string) (int, error) {
	var err error
	return 0, err
}