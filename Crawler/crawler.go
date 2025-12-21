package crawler

import (
	"log"
	"strconv"
	"strings"

	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)
var Colly *colly.Collector

func init(){
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
}


func GetPrice(uri string, querySelector string) (int, error) {
	var err error
	res := 0
	log.Println("logging url", uri)
	Colly.OnHTML(querySelector, func(h *colly.HTMLElement) {
		ret := strings.ReplaceAll(h.Text, "$", "")
		ret = strings.TrimSpace(ret)
		ret = strings.Split(ret, ".")[0]
		res, err = strconv.Atoi(ret)
		log.Println("got price from:", h.Text)
		log.Println("price", res, ret, err)
	})
	err = Colly.Visit(uri)
	
	Colly.Wait()
	if err != nil{
		log.Println("error in getting price in crawler", err)
		return res, err
	}
	return res, err
}