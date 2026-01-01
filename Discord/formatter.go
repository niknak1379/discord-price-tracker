package discord

import (
	"fmt"
	database "priceTracker/Database"
	types "priceTracker/Types"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

func setEmbed(Item *database.Item) []*discordgo.MessageEmbed {
	var fields []*discordgo.MessageEmbedField
	var retArr []*discordgo.MessageEmbed
	// set up current price information
	trackerFields := setTrackerFields(Item)
	ebayFields := setSecondHandField(Item.EbayListings)
	priceFields := setPriceField(&Item.CurrentLowestPrice, "Current")
	lowestPriceField := setPriceField(&Item.LowestPrice, "Historically Lowest")
	fields = append(fields, trackerFields...)
	fields = append(fields, ebayFields...)
	fields = append(fields, priceFields...)
	fields = append(fields, lowestPriceField...)
	fmt.Println("total field length for", Item.Name, len(fields))
	// apparently go doesnt have ceil for int division
	// its either float conversions or this
	for i := range (len(fields)+ 20)/21{
		endInd := len(fields) - 1
		if (i + 1) * 21 < endInd {
			endInd = (i + 1) * 21
		}
		em := discordgo.MessageEmbed{
			Title: Item.Name,
			Image: &discordgo.MessageEmbedImage{
				URL:    Item.ImgURL,
				Height: 300,
				Width:  300,
			},
			Fields: fields[i*21:endInd + 1],
			Type:   discordgo.EmbedTypeRich,
		}
		retArr = append(retArr, &em)
		fmt.Println("setting embed for indexes ", i * 21, endInd, len(fields))
	}
	return retArr
}
func setTrackerFields(Item *database.Item)[]*discordgo.MessageEmbedField{
	var fields []*discordgo.MessageEmbedField
	// set up trackerArr infromation
	field := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("Tracking URL", 43),
		Value:  embedSeparatorFormatter("CSS Selector", 44),
		Inline: false,
	}
	fields = append(fields, &field)
	for _, tracker := range Item.TrackingList {
		field := discordgo.MessageEmbedField{
			Name:   tracker.URI,
			Value:  tracker.HtmlQuery,
			Inline: false,
		}
		separatorField := discordgo.MessageEmbedField{
			Name:   embedSeparatorFormatter("", 45),
			Value:  "",
			Inline: false,
		}
		fields = append(fields, &field, &separatorField)
	}
	return fields
}
func setSecondHandField(ebayArr []types.EbayListing) []*discordgo.MessageEmbedField {
	var res []*discordgo.MessageEmbedField
	if len(ebayArr) == 0 {
		return res
	}
	HeaderField := discordgo.MessageEmbedField{
		Name: embedSeparatorFormatter("Ebay Listings", 44),
	}
	res = append(res, &HeaderField)
	for _, Listing := range ebayArr {
		listFields := formatSecondHandField(Listing)
		res = append(res, listFields...)
	}
	return res
}
func formatSecondHandField (Listing types.EbayListing) []*discordgo.MessageEmbedField{
	priceField := discordgo.MessageEmbedField{
		Name:  Listing.Title,
		Value: "$" + strconv.Itoa(Listing.Price+1),
		Inline: false,
	}
	conditionField := discordgo.MessageEmbedField{
		Name:  "Condition/Location:",
		Value: Listing.Condition,
		Inline: false,
	}
	urlField := discordgo.MessageEmbedField{
		Name:  "From URL:",
		Value: Listing.URL,
		Inline: false,
	}
	separatorField := discordgo.MessageEmbedField{
		Name:  embedSeparatorFormatter("", 44),
		Value: "",
		Inline: false,
	}
	var ret []*discordgo.MessageEmbedField
	return append(ret, &priceField, &conditionField, &urlField, &separatorField)
}
func setPriceField(p *database.Price, message string) []*discordgo.MessageEmbedField {
	priceField := discordgo.MessageEmbedField{
		Name: embedSeparatorFormatter(fmt.Sprintf("%s Price", message), 44),
		Value: func() string {
			if p.Price == 0 {
				return "Item Unavailable"
			} else {
				return "$" + strconv.Itoa(p.Price+1)
			}
		}(),
		Inline: false,
	}
	urlField := discordgo.MessageEmbedField{
		Name:   "From Price Source:",
		Value:  p.Url,
		Inline: false,
	}
	dateField := discordgo.MessageEmbedField{
		Name: "Date:",
		Value: p.Date.Format("2006-01-02"),
		Inline: false,
	}
	fmt.Println(p.Date.Format("2006-01-02"))
	var fields []*discordgo.MessageEmbedField
	fields = append(fields, &priceField, &urlField, &dateField)
	return fields
}

// <-------- s --------->
// formats strings like above and total output string length of l
func embedSeparatorFormatter(s string, l int) string {
	flip := false
	initLen := len(s)
	if initLen > l {
		return s
	}

	for i := 0; i < (l - initLen); i++ {
		if i == l-initLen-2 {
			s = "<" + s
		} else if i == l-initLen-1 {
			s = s + ">"
		} else if flip {
			s = "-" + s
		} else {
			s = s + "-"
		}
		flip = !flip
	}
	return s
}
