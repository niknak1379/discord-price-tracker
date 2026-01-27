package discord

import (
	"fmt"
	"strconv"

	database "priceTracker/Database"
	types "priceTracker/Types"

	"github.com/bwmarrin/discordgo"
)

const (
	MaxFieldsPerEmbed = 25   // Discord limit
	MaxFieldNameLen   = 256  // Discord limit
	MaxFieldValueLen  = 1024 // Discord limit
	MaxEmbedSize      = 6000 // Total characters across all fields
)

func setEmbed(Item *database.Item) []*discordgo.MessageEmbed {
	var fields []*discordgo.MessageEmbedField
	var retArr []*discordgo.MessageEmbed

	// Set up current price information
	trackerFields := setTrackerFields(Item)
	ebayFields := setSecondHandField(Item.EbayListings)
	aggregatefields := formatAggregateFields(Item.SevenDayAggregate, "Used Aggregation For the Last 7 Days")
	priceFields := setPriceField(&Item.CurrentLowestPrice, "Current")
	lowestPriceField := setPriceField(&Item.LowestPrice, "Historically Lowest")

	fields = append(fields, trackerFields...)
	fields = append(fields, ebayFields...)
	fields = append(fields, aggregatefields...)
	fields = append(fields, priceFields...)
	fields = append(fields, lowestPriceField...)

	// Split fields into embeds based on Discord limits
	currentFields := []*discordgo.MessageEmbedField{}
	currentSize := 0

	for _, field := range fields {
		fieldSize := len(field.Name) + len(field.Value)

		// Check if adding this field would exceed limits
		shouldCreateNewEmbed := len(currentFields) >= MaxFieldsPerEmbed ||
			currentSize+fieldSize > MaxEmbedSize

		if shouldCreateNewEmbed && len(currentFields) > 0 {
			// Create embed with current fields
			em := &discordgo.MessageEmbed{
				Title: Item.Name,
				Image: &discordgo.MessageEmbedImage{
					URL:    Item.ImgURL,
					Height: 300,
					Width:  300,
				},
				Fields: currentFields,
				Type:   discordgo.EmbedTypeRich,
			}
			retArr = append(retArr, em)

			// Reset for next embed
			currentFields = []*discordgo.MessageEmbedField{}
			currentSize = 0
		}

		// Add field to current embed
		currentFields = append(currentFields, field)
		currentSize += fieldSize
	}

	// Add remaining fields as final embed
	if len(currentFields) > 0 {
		em := &discordgo.MessageEmbed{
			Title: Item.Name,
			Image: &discordgo.MessageEmbedImage{
				URL:    Item.ImgURL,
				Height: 300,
				Width:  300,
			},
			Fields: currentFields,
			Type:   discordgo.EmbedTypeRich,
		}
		retArr = append(retArr, em)
	}

	return retArr
}

func setTrackerFields(Item *database.Item) []*discordgo.MessageEmbedField {
	var fields []*discordgo.MessageEmbedField

	// Set up trackerArr information
	field := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("Tracking URL", 43),
		Value:  embedSeparatorFormatter("CSS Selector", 44),
		Inline: false,
	}
	fields = append(fields, &field)

	for _, tracker := range Item.TrackingList {
		field := discordgo.MessageEmbedField{
			Name:   truncateString(tracker.URI, MaxFieldNameLen),
			Value:  truncateString(tracker.HtmlQuery, MaxFieldValueLen),
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
		listFields := formatSecondHandField(Listing, "Price", true)
		res = append(res, listFields...)
	}
	return res
}

// new listing is there so that it  doesnt return duplicate fields in discord.response
// for alerts
func formatSecondHandField(Listing types.EbayListing, Message string, newListing bool) []*discordgo.MessageEmbedField {
	var ret []*discordgo.MessageEmbedField
	AcceptsOffer := "No"
	if Listing.AcceptsOffers {
		AcceptsOffer = "Yes"
	}
	currOrOld := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter(Message, 43),
		Value:  "",
		Inline: false,
	}
	priceField := discordgo.MessageEmbedField{
		Name:   truncateString(Listing.Title, MaxFieldNameLen),
		Value:  "$" + strconv.Itoa(Listing.Price+1),
		Inline: false,
	}
	conditionField := discordgo.MessageEmbedField{
		Name:   "Condition/Location:",
		Value:  truncateString(Listing.Condition, MaxFieldValueLen),
		Inline: false,
	}
	urlField := discordgo.MessageEmbedField{
		Name:   "From URL:",
		Value:  truncateString(Listing.URL, MaxFieldValueLen),
		Inline: false,
	}
	separatorField := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("", 44),
		Value:  "",
		Inline: false,
	}
	if newListing {
		durationField := discordgo.MessageEmbedField{
			Value: strconv.Itoa(int(Listing.Duration.Hours()/24)) + " Days and " +
				strconv.Itoa(int(Listing.Duration.Hours())%24) + " Hours",
			Name:   "Listing Duration:",
			Inline: false,
		}
		priceDecField := discordgo.MessageEmbedField{
			Name:   "# of Price Decreases:",
			Value:  strconv.Itoa(Listing.PriceDecreaseNum),
			Inline: false,
		}
		AcceptsOffer := discordgo.MessageEmbedField{
			Name:   "Does Listing Accept Offers",
			Value:  AcceptsOffer,
			Inline: false,
		}
		priceIncField := discordgo.MessageEmbedField{
			Name:   "# of Price Increases:",
			Value:  strconv.Itoa(Listing.PriceIncreaseNum),
			Inline: false,
		}
		totalPriceChange := discordgo.MessageEmbedField{
			Name:   "Total Price Change $",
			Value:  strconv.Itoa(Listing.TotalPriceChange),
			Inline: false,
		}
		return append(ret, &currOrOld, &priceField, &AcceptsOffer, &conditionField, &urlField,
			&durationField, &priceDecField, &priceIncField, &totalPriceChange, &separatorField)
	}

	return append(ret, &currOrOld, &priceField,
		&separatorField)
}

func formatAggregateFields(Aggregate database.AggregateReport, message string) []*discordgo.MessageEmbedField {
	/*
		struct {
		UniqueListings              int `bson:"UniqueListings"`
		AverageDaysUP               int `bson:"AverageDaysUP"`
		AveragePrice                int `bson:"AveragePrice"`
		PriceSTDEV                  int `bson:"PriceSTDEV"`
		AveragePriceWhenSold        int `bson:"AveragePriceWhenSold"`
		LowestPriceDuringTimePeriod int `bson:"LowestPriceDuringTimePeriod"`
	}*/
	Message := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter(message, 43),
		Value:  "",
		Inline: false,
	}
	uniqueLitings := discordgo.MessageEmbedField{
		Name:   "Unique Listings:",
		Value:  strconv.Itoa(Aggregate.UniqueListings),
		Inline: false,
	}
	AverageDuration := discordgo.MessageEmbedField{
		Name:   "Avergae Duration Of Listing:",
		Value:  strconv.Itoa(Aggregate.AverageDaysUP),
		Inline: false,
	}
	AveragePrice := discordgo.MessageEmbedField{
		Name:   "Avergae Price Of Listing:",
		Value:  "$ " + strconv.Itoa(Aggregate.AveragePrice),
		Inline: false,
	}
	AveragePriceWhenSold := discordgo.MessageEmbedField{
		Name:   "Avergae Price Of Listing When Sold:",
		Value:  "$ " + strconv.Itoa(Aggregate.AveragePriceWhenSold),
		Inline: false,
	}
	STDEV := discordgo.MessageEmbedField{
		Name:   "STDEV of Prices:",
		Value:  "$ " + strconv.Itoa(Aggregate.PriceSTDEV),
		Inline: false,
	}
	LowestPriceDuringTimePeriod := discordgo.MessageEmbedField{
		Name:   "Lowest Price During Time Period:",
		Value:  "$ " + strconv.Itoa(Aggregate.LowestPriceDuringTimePeriod),
		Inline: false,
	}
	SeparatorField := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("", 44),
		Value:  "",
		Inline: false,
	}
	var res []*discordgo.MessageEmbedField
	res = append(res, &Message, &uniqueLitings, &AverageDuration, &AveragePrice, &AveragePriceWhenSold, &STDEV, &LowestPriceDuringTimePeriod, &SeparatorField)
	return res
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
		Value:  truncateString(p.Url, MaxFieldValueLen),
		Inline: false,
	}
	dateField := discordgo.MessageEmbedField{
		Name:   "Date:",
		Value:  p.Date.Format("2006-01-02"),
		Inline: false,
	}

	var fields []*discordgo.MessageEmbedField
	fields = append(fields, &priceField, &urlField, &dateField)
	return fields
}

func formatChannelInfo(Channel *database.Channel) *discordgo.MessageEmbed {
	locationField := discordgo.MessageEmbedField{
		Name:   "Facebook Locaiton Code",
		Value:  Channel.LocationCode,
		Inline: false,
	}
	ChannelIDField := discordgo.MessageEmbedField{
		Name:   "Channel ID",
		Value:  Channel.ChannelID,
		Inline: false,
	}
	distanceField := discordgo.MessageEmbedField{
		Name:   "Max Distance",
		Value:  strconv.Itoa(Channel.Distance),
		Inline: false,
	}
	totalItemField := discordgo.MessageEmbedField{
		Name:   "Total Items",
		Value:  strconv.Itoa(Channel.TotalItems),
		Inline: false,
	}
	em := &discordgo.MessageEmbed{
		Title:  "Channel Information",
		Fields: []*discordgo.MessageEmbedField{&ChannelIDField, &totalItemField, &locationField, &distanceField},
	}
	return em
}

// Truncate string to max length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
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
