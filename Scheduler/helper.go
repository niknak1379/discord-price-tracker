package scheduler

import database "priceTracker/Database"

// HaveItemPropertiesChanged checks if any item properties have changed between two versions.
//
// Parameters:
//   - currItem: the current item state
//   - oldItem: the previous item state
//
// Returns true if any properties have changed.
func HaveItemPropertiesChanged(currItem, oldItem *database.Item) bool {
	if currItem.SuppressNotifications != oldItem.SuppressNotifications ||
		currItem.Timer != oldItem.Timer ||
		len(currItem.AlternateTrackingQueries) != len(oldItem.AlternateTrackingQueries) ||
		len(currItem.TrackingExclusionQueries) != len(oldItem.TrackingExclusionQueries) ||
		currItem.CurrentLowestPrice.Price != oldItem.CurrentLowestPrice.Price ||
		currItem.FacebookCrawl != oldItem.FacebookCrawl {
		return true
	}
	// check weather tracking list was changed
	if len(currItem.TrackingList) == len(oldItem.TrackingList) {
		for index := range currItem.TrackingList {
			if currItem.TrackingList[index].HtmlQuery != oldItem.TrackingList[index].HtmlQuery ||
				currItem.TrackingList[index].URI != oldItem.TrackingList[index].URI {
				return true
			}
		}
	} else {
		return true
	}
	return false
}
