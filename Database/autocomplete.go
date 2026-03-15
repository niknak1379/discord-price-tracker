package database

import "github.com/lithammer/fuzzysearch/fuzzy"

// this was deprecated from moving the database off atlas
// local mongodb doesnt support search vectors so have to use
// local fuzzy algorithms in the binary itself
//
// FuzzyMatchName performs fuzzy search on item names for autocomplete suggestions.
//
// Parameters:
//   - Name: the partial name to search for (empty string returns all names)
//   - ChannelID: the Discord channel ID
//
// Returns a list of matching item names.
//
//	func FuzzyMatchName(Name string, ChannelID string) []string {
//		Table, err := loadChannelTable(ChannelID)
//		if err != nil {
//			slog.Error("couldnt load channel", slog.Any("Error", err))
//			return make([]string, 0)
//		}
//
//		projectStage := bson.D{{Key: "$project", Value: bson.D{{Key: "Name", Value: 1}}}}
//		var pipeline mongo.Pipeline
//
//		if Name == "" {
//			pipeline = mongo.Pipeline{
//				bson.D{{Key: "$match", Value: bson.D{}}},
//				bson.D{{Key: "$sort", Value: bson.D{{Key: "Name", Value: 1}}}},
//				projectStage,
//			}
//		} else {
//			pipeline = mongo.Pipeline{
//				bson.D{{Key: "$search", Value: bson.D{
//					{Key: "index", Value: ChannelID},
//					{Key: "autocomplete", Value: bson.D{
//						{Key: "path", Value: "Name"},
//						{Key: "query", Value: Name},
//						{Key: "fuzzy", Value: bson.D{
//							{Key: "maxEdits", Value: 2},
//							{Key: "prefixLength", Value: 1},
//						}},
//					}},
//				}}},
//				projectStage,
//			}
//		}
//
//		cursor, err := Table.Aggregate(ctx, pipeline)
//		if err != nil {
//			slog.Error("Error", slog.Any("Error", err))
//		}
//		defer cursor.Close(ctx)
//		names := make([]string, 0)
//
//		for cursor.Next(ctx) {
//			var result bson.M
//			if err := cursor.Decode(&result); err != nil {
//				slog.Error("Error", slog.Any("Error", err))
//				continue
//			}
//
//			if name, ok := result["Name"].(string); ok {
//				names = append(names, name)
//			}
//		}
//
//		if err := cursor.Err(); err != nil {
//			slog.Error("Error", slog.Any("Error", err))
//		}
//
//		return names
//	}

// FuzzyMatchName performs fuzzy search on item names for autocomplete suggestions.
//
// Parameters:
//   - Name: the partial name to search for (empty string returns all names)
//   - ChannelID: the Discord channel ID
//
// Returns a list of matching item names.
func FuzzyMatchName(Name string, ChannelID string) []string {
	itemsArr := GetAllItems(ChannelID,
		[]string{"PriceHistory", "ListingsHistory", "EbayListings", "EbayBids"})
	NameArr := []string{}
	for _, Item := range itemsArr {
		NameArr = append(NameArr, Item.Name)
	}
	if len(NameArr) == 0 {
		NameArr = append(NameArr, "No Results Found :0")
	}

	return fuzzy.FindFold(Name, NameArr)
}

// not really critical functionality i feel like i dont really
// need to propogate the errors for this and the other autocomplete
// AutoCompleteURL retrieves tracking URLs for an item for autocomplete.
//
// Parameters:
//   - Name: the item name
//   - ChannelID: the Discord channel ID
//
// Returns a list of tracking URLs for the item.
func AutoCompleteURL(Name string, ChannelID string) []string {
	item, err := GetItem(Name, ChannelID, "PriceHistory", "ListingsHistory", "EbayListings", "EbayBids")
	res := []string{}
	if err != nil {
		return res
	}
	for _, tracker := range item.TrackingList {
		res = append(res, tracker.URI)
	}
	if len(res) == 0 {
		res = append(res, "No Results Found :0")
	}
	return res
}

// AutoCompleteAlternateTrackingQueries retrieves alternate tracking names for autocomplete.
//
// Parameters:
//   - Name: the item name
//   - ChannelID: the Discord channel ID
//
// Returns a list of alternate tracking names for the item.
func AutoCompleteAlternateTrackingQueries(Name string, ChannelID string) []string {
	item, err := GetItem(Name, ChannelID, "PriceHistory", "ListingsHistory", "EbayListings", "EbayBids")
	res := []string{}
	if err != nil {
		return res
	}
	for _, query := range item.AlternateTrackingQueries {
		res = append(res, query)
	}
	if len(res) == 0 {
		res = append(res, "No Results Found :0")
	}
	return res
}

// AutoCompleteTrackingExclusionQueries retrieves exclusion queries for autocomplete.
//
// Parameters:
//   - Name: the item name
//   - ChannelID: the Discord channel ID
//
// Returns a list of exclusion queries for the item.
func AutoCompleteTrackingExclusionQueries(Name string, ChannelID string) []string {
	item, err := GetItem(Name, ChannelID, "PriceHistory", "ListingsHistory", "EbayListings", "EbayBids")
	res := []string{}
	if err != nil {
		return res
	}
	for _, query := range item.TrackingExclusionQueries {
		res = append(res, query)
	}
	if len(res) == 0 {
		res = append(res, "No Results Found :0")
	}
	return res
}

// AutoCompleteQuery returns a map of CSS query selectors for common websites.
//
// Returns a map of domain names to their price CSS selectors.
func AutoCompleteQuery() map[string]string {
	ret := map[string]string{
		//div[data-feature-name='corePriceDisplay_desktop'] span.priceToPay
		//"Amazon":       "form#addToCart span.a-price-whole",
		// for products that also have a used listig
		"Amazon_Default": "div#apex_desktop_newAccordionRow span.priceToPay",
		// for products that dont have used listings
		"Amazon_Backup": "div#apex_desktop span.priceToPay",
		"NewEgg":        "div.price-current>strong",
		"MicroCenter":   "#options-pricing2022",
		"BHPhotoVideo":  "span[class^='price_']",
		// "BestBuy":      "div[data-component-name='LargePrice'] div[data-testid='price-block-customer-price']",
		// "BestBuy": "div[data-testid='price-block-customer-price']",
		"BestBuy": "div[data-component-name='StickyProductHeader'] div[data-testid='price-block-customer-price']",
	}
	return ret
}
