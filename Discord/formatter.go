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

// setEmbed creates Discord embed(s) for displaying item information.
// Splits fields across multiple embeds if Discord limits are exceeded.
//
// Parameters:
//   - Item: the item to display
//
// Returns a slice of Discord embeds.
func setEmbed(Item *database.Item) []*discordgo.MessageEmbed {
	var fields []*discordgo.MessageEmbedField
	var retArr []*discordgo.MessageEmbed

	// Set up current price information
	alternativeNameFields := setAlternateNamesFields(Item)
	trackerFields := setTrackerFields(Item)
	ebayFields := setSecondHandField(Item.EbayListings)
	aggregatefields := formatAggregateFields(&Item.SevenDayAggregate, "Used Aggregation For the Last 7 Days")
	priceFields := setPriceField(&Item.CurrentLowestPrice, "Current")
	lowestPriceField := setPriceField(&Item.LowestPrice, "Historically Lowest")
	bidFields := setBidFields(Item.EbayBids)

	fields = append(fields, alternativeNameFields...)
	fields = append(fields, trackerFields...)
	fields = append(fields, ebayFields...)
	fields = append(fields, bidFields...)
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

// setAlternateNamesFields creates fields for alternate tracking names and exclusion queries.
// Also includes the timer field.
//
// Parameters:
//   - Item: the item containing alternate names and exclusion queries
//
// Returns a slice of Discord embed fields.
func setAlternateNamesFields(Item *database.Item) []*discordgo.MessageEmbedField {
	Fields := []*discordgo.MessageEmbedField{}
	// im also gonna set timer fields here too
	if Item.Timer == 0 {
		Item.Timer = 8
	}
	timer := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("Timer", 43),
		Value:  strconv.Itoa(Item.Timer) + " h",
		Inline: false,
	}
	Fields = append(Fields, &timer)
	for _, Name := range Item.AlternateTrackingQueries {
		field := discordgo.MessageEmbedField{
			Name:   embedSeparatorFormatter("Alternative Name", 43),
			Value:  Name,
			Inline: false,
		}
		Fields = append(Fields, &field)
	}
	for _, query := range Item.TrackingExclusionQueries {
		field := discordgo.MessageEmbedField{
			Name:   embedSeparatorFormatter("Exclusion Query", 43),
			Value:  query,
			Inline: false,
		}
		Fields = append(Fields, &field)
	}
	return Fields
}

// setTrackerFields creates fields for tracking URLs and their CSS selectors.
//
// Parameters:
//   - Item: the item containing tracking information
//
// Returns a slice of Discord embed fields.
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

// setSecondHandField creates fields for eBay second-hand listings.
//
// Parameters:
//   - ebayArr: slice of eBay listings to display
//
// Returns a slice of Discord embed fields.
func setSecondHandField(ebayArr []*types.EbayListing) []*discordgo.MessageEmbedField {
	var res []*discordgo.MessageEmbedField
	if len(ebayArr) == 0 {
		return res
	}

	HeaderField := discordgo.MessageEmbedField{
		Name: embedSeparatorFormatter("Ebay Listings", 44),
	}
	res = append(res, &HeaderField)

	for _, Listing := range ebayArr {
		listFields := formatSecondHandField(Listing, "Price", true, true)
		res = append(res, listFields...)
	}
	return res
}

// setBidFields creates fields for eBay bids.
//
// Parameters:
//   - ebayArr: slice of eBay bids to display
//
// Returns a slice of Discord embed fields.
func setBidFields(ebayArr []*types.EbayBids) []*discordgo.MessageEmbedField {
	bidFields := []*discordgo.MessageEmbedField{}
	if len(ebayArr) == 0 {
		return bidFields
	}
	HeaderField := discordgo.MessageEmbedField{
		Name: embedSeparatorFormatter("Ebay Bids", 44),
	}
	bidFields = append(bidFields, &HeaderField)

	for _, Listing := range ebayArr {
		listFields := formatBidField(Listing, false)
		bidFields = append(bidFields, listFields...)
	}

	return bidFields
}

// formatBidField creates formatted fields for a single eBay bid.
//
// Parameters:
//   - Listing: the bid to format
//   - priceChange: whether this is for a price change alert (old vs new price)
//
// Returns a slice of Discord embed fields.
func formatBidField(Listing *types.EbayBids, priceChange bool) []*discordgo.MessageEmbedField {
	retArr := []*discordgo.MessageEmbedField{}
	NameField := discordgo.MessageEmbedField{
		Name:   Listing.Title,
		Value:  "",
		Inline: false,
	}
	Price := discordgo.MessageEmbedField{
		Name:   "Current Price",
		Value:  "$ " + strconv.Itoa(Listing.Price),
		Inline: false,
	}
	separatorField := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("", 44),
		Value:  "",
		Inline: false,
	}
	if priceChange {
		Price.Name = "Old Price"
		return append(retArr, &Price)
	}
	BidNumber := discordgo.MessageEmbedField{
		Name:   "Number of Bids",
		Value:  strconv.Itoa(Listing.Bids),
		Inline: false,
	}
	EndDate := discordgo.MessageEmbedField{
		Name: "End Date",
		Value: Listing.EndDate.Weekday().String() +
			" " + Listing.EndDate.Format("3:04PM"),
		Inline: false,
	}
	conditionField := discordgo.MessageEmbedField{
		Name:   "Condition:",
		Value:  truncateString(Listing.Condition, MaxFieldValueLen),
		Inline: false,
	}
	urlField := discordgo.MessageEmbedField{
		Name:   "From URL:",
		Value:  truncateString(Listing.URL, MaxFieldValueLen),
		Inline: false,
	}
	return append(retArr, &NameField, &BidNumber, &EndDate, &conditionField, &urlField, &Price, &separatorField)
}

// formatSecondHandField creates formatted fields for a second-hand listing.
// Optimized to avoid duplicate fields for Discord response alerts.
//
// Parameters:
//   - Listing: the eBay listing to format
//   - Message: the message header (e.g., "Price", "Old Price")
//   - newListing: whether this is a new listing alert
//   - priceChange: whether this is a price change alert
//
// Returns a slice of Discord embed fields.
func formatSecondHandField(
	Listing *types.EbayListing,
	Message string,
	newListing bool,
	priceChange bool,
) []*discordgo.MessageEmbedField {
	var ret []*discordgo.MessageEmbedField
	AcceptsOfferStr := "No"
	if Listing.AcceptsOffers {
		AcceptsOfferStr = "Yes"
	}
	currOrOld := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter(Message, 43),
		Value:  "",
		Inline: false,
	}
	titleField := discordgo.MessageEmbedField{
		Value:  truncateString(Listing.Title, MaxFieldNameLen),
		Name:   "Title",
		Inline: false,
	}
	priceField := discordgo.MessageEmbedField{
		Name:   "Price",
		Value:  "$" + strconv.Itoa(Listing.Price+1),
		Inline: false,
	}
	separatorField := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("", 44),
		Value:  "",
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
		Value:  AcceptsOfferStr,
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
	if newListing && !priceChange {
		return append(ret, &currOrOld, &titleField, &AcceptsOffer, &conditionField, &urlField,
			&priceField, &separatorField)
	} else if newListing && priceChange {
		return append(ret, &currOrOld, &titleField, &AcceptsOffer, &conditionField, &urlField,
			&durationField, &priceDecField, &priceIncField, &totalPriceChange, &priceField, &separatorField)
	} else { // if old Listing
		return append(ret, &currOrOld, &priceField,
			&separatorField)
	}
}

// formatAggregateFields creates fields for aggregate report statistics.
//
// Parameters:
//   - Aggregate: the aggregate report containing statistics
//   - message: the header message for the field group
//
// Returns a slice of Discord embed fields.
func formatAggregateFields(Aggregate *database.AggregateReport, message string) []*discordgo.MessageEmbedField {
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

// setPriceField creates fields for displaying a price with source URL and date.
//
// Parameters:
//   - p: the price to display
//   - message: the message prefix (e.g., "Current", "Historically Lowest")
//
// Returns a slice of Discord embed fields.
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

// formatChannelInfo creates an embed displaying channel configuration information.
//
// Parameters:
//   - Channel: the channel configuration to display
//
// Returns a Discord embed.
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

// truncateString truncates a string to max length with ellipsis.
//
// Parameters:
//   - s: the string to truncate
//   - maxLen: the maximum length of the output string
//
// Returns the truncated string.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// embedSeparatorFormatter formats a string with dashes on both sides for visual separation.
// Formats strings like "<-------- s --------->" where the total length is l.
//
// Parameters:
//   - s: the string to format
//   - l: the total length of the output string
//
// Returns the formatted separator string.
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
