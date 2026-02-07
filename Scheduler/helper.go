package scheduler

import database "priceTracker/Database"

func HaveItemPropertiesChanged(currItem, oldItem *database.Item) bool {
	if currItem.SuppressNotifications != oldItem.SuppressNotifications ||
		currItem.Timer != oldItem.Timer ||
		len(currItem.AlternateTrackingQueries) != len(oldItem.AlternateTrackingQueries) ||
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
