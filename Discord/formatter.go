package discord

import (
	"fmt"
	database "priceTracker/Database"
	types "priceTracker/Types"
	"strconv"

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
	priceFields := setPriceField(&Item.CurrentLowestPrice, "Current")
	lowestPriceField := setPriceField(&Item.LowestPrice, "Historically Lowest")

	fields = append(fields, trackerFields...)
	fields = append(fields, ebayFields...)
	fields = append(fields, priceFields...)
	fields = append(fields, lowestPriceField...)

	fmt.Println("total field length for", Item.Name, len(fields))

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
			fmt.Printf("Created embed with %d fields, %d chars\n", len(currentFields), currentSize)

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
		fmt.Printf("Created final embed with %d fields, %d chars\n", len(currentFields), currentSize)
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
		listFields := formatSecondHandField(Listing, "Price")
		res = append(res, listFields...)
	}
	return res
}

func formatSecondHandField(Listing types.EbayListing, Message string) []*discordgo.MessageEmbedField {
	currOrOld := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("Message", 44),
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

	var ret []*discordgo.MessageEmbedField
	return append(ret, &currOrOld, &priceField, &conditionField, &urlField, &separatorField)
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
	fmt.Println(p.Date.Format("2006-01-02"))

	var fields []*discordgo.MessageEmbedField
	fields = append(fields, &priceField, &urlField, &dateField)
	return fields
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

