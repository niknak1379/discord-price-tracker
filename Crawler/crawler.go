package crawler

import (
	"context"
	"log"
	"math/rand/v2"
	database "priceTracker/Database"
	"strconv"

	discord "priceTracker/Discord"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)
var Colly *colly.Collector

func InitCrawler(ctx context.Context, cancel context.CancelFunc){
	log.Println("initalizing crawler")
	// --------------------------- initiaize scrapper headers and settings ------- //
	
	Colly = colly.NewCollector(
		colly.MaxDepth(1),
        colly.AllowURLRevisit(),
	)
	Colly.SetRequestTimeout(30 * time.Second)
	Colly.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       2 * time.Second,  // Wait 2 seconds between requests
		RandomDelay: 1 * time.Second,  // Add random delay (1-3 seconds total)
	})
	extensions.RandomUserAgent(Colly)
	Colly.OnRequest(func(r *colly.Request) {
        r.Headers.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
        r.Headers.Set("Accept-Language", "en-US,en;q=0.9")
        r.Headers.Set("Accept-Encoding", "gzip, deflate, br")
        r.Headers.Set("DNT", "1")
        r.Headers.Set("Connection", "keep-alive")
        r.Headers.Set("Upgrade-Insecure-Requests", "1")
        r.Headers.Set("Sec-Fetch-Dest", "document")
        r.Headers.Set("Sec-Fetch-Mode", "navigate")
        r.Headers.Set("Sec-Fetch-Site", "cross-site")
        r.Headers.Set("Referer", "https://www.google.com/")
    })
	Colly.OnError(func(r *colly.Response, err error) {
        log.Printf("Error scraping %s: %v", r.Request.URL, err)
    })

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
	newPrice, err := getPrice(URI, HtmlQuery)
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
func getPrice(uri string, querySelector string) (int, error) {
	var err error
	res := 0
	Colly.OnHTML(querySelector, func(h *colly.HTMLElement) {
		res, err = strconv.Atoi(h.Text)
	})
	err = Colly.Visit(uri)
	
	Colly.Wait()
	if err != nil{
		log.Println("error in getting price in crawler", err)
		return res, err
	}
	return res, err
}