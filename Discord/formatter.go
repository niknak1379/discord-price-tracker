package discord

import (
	"fmt"
	database "priceTracker/Database"
	types "priceTracker/Types"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

func setEmbed(Item database.Item) *discordgo.MessageEmbed {
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

	// set up current price information
	ebayFields := setSecondHandField(Item.EbayListings)
	priceFields := setPriceField(Item.CurrentLowestPrice, "Current")
	lowestPriceField := setPriceField(Item.LowestPrice, "Historically Lowest")
	fields = append(fields, ebayFields...)
	fields = append(fields, priceFields...)
	fields = append(fields, lowestPriceField...)
	em := discordgo.MessageEmbed{
		Title: Item.Name,
		Image: &discordgo.MessageEmbedImage{
			URL:    Item.ImgURL,
			Height: 300,
			Width:  300,
		},
		Fields: fields,
		Type:   discordgo.EmbedTypeRich,
	}
	fmt.Println(Item.ImgURL)
	return &em
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
		priceField := discordgo.MessageEmbedField{
			Name:  Listing.Title,
			Value: "$" + strconv.Itoa(Listing.Price+1),
		}
		conditionField := discordgo.MessageEmbedField{
			Name:  "Condition/Location:",
			Value: Listing.Condition,
		}
		urlField := discordgo.MessageEmbedField{
			Name:  "From URL:",
			Value: Listing.URL,
		}
		separatorField := discordgo.MessageEmbedField{
			Name:  embedSeparatorFormatter("", 44),
			Value: "",
		}
		res = append(res, &priceField, &conditionField, &urlField, &separatorField)
	}
	return res
}

func setPriceField(p database.Price, message string) []*discordgo.MessageEmbedField {
	/* separatorField := discordgo.MessageEmbedField{
		Name: "<------------------------------------------->",
		Value: "",
		Inline: false,
	} */
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
	var fields []*discordgo.MessageEmbedField
	fields = append(fields, &priceField, &urlField)
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
