package discord

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"strings"
	"time"

	database "priceTracker/Database"
	types "priceTracker/Types"

	"github.com/bwmarrin/discordgo"
)

// color variables

var (
	red        = 10038562 // red
	aqua       = 1752220  // aqua
	darkGold   = 12745742 // dark gold
	blue       = 3447003
	yellow     = 16776960 // yellow
	extremeRed = 16711680 // very red
	pink       = 15277667 // pink
	green      = 2067276  // green
	orange     = 16737380
	razerGreen = 3328050
)

func ready(discord *discordgo.Session, ready *discordgo.Ready) {
	slog.Info("Discord Logged in")
	discord.UpdateGameStatus(1, "stonks")
}

// PriceChangeAlert sends a Discord embed notification when an item's price changes.
//
// Parameters:
//   - itemName: the name of the item
//   - newPrice: the new price value
//   - oldPrice: the previous price
//   - URL: the URL of the product
//   - ChannelID: the Discord channel ID
func PriceChangeAlert(itemName string, newPrice int, oldPrice database.Price, URL string, ChannelID string) {
	var color int
	if newPrice > oldPrice.Price {
		color = yellow // yellow
	} else {
		color = green // green
	}
	oldPriceField := setPriceField(&oldPrice, "Previous ")
	newPriceField := setPriceField(&database.Price{
		Price: newPrice,
		Url:   URL,
		Date:  time.Now(),
	}, "New ")
	var Fields []*discordgo.MessageEmbedField
	Fields = append(Fields, oldPriceField...)
	Fields = append(Fields, newPriceField...)
	em := discordgo.MessageEmbed{
		Title:       "Price Update",
		Description: itemName,
		Color:       color,
		URL:         URL,
		Fields:      Fields,
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

// CrawlErrorAlert sends a Discord embed notification when a crawler error occurs.
//
// Parameters:
//   - itemName: the name of the item
//   - URL: the URL being crawled
//   - err: the error that occurred
//   - ChannelID: the Discord channel ID
func CrawlErrorAlert(itemName string, URL string, err error, ChannelID string) {
	var s string
	if err != nil {
		s = fmt.Sprintf("Crawler could not find price for %s in url %s, with error %s investigate logs for further information",
			itemName, URL, err.Error())
	} else {
		s = fmt.Sprintf("returned price of 0 for item %s, with url %s", itemName, URL)
	}
	slog.Error(s)
	nameField := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("Problematic Item", 42),
		Value:  itemName,
		Inline: false,
	}
	urlField := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("Problematic URL", 42),
		Value:  URL,
		Inline: false,
	}

	// character limit for each field is 1024 but i dont know how thats gonna go with other fields
	maxLen := int(math.Min(float64(len(err.Error())), 1023))

	errField := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("Error Message", 43),
		Value:  err.Error()[:maxLen],
		Inline: false,
	}
	var Fields []*discordgo.MessageEmbedField
	Fields = append(Fields, &nameField, &urlField, &errField)
	//
	// <--------------- send screenshots of failed crawl --------->
	if strings.Contains(err.Error(), "facebook") {
		reader, err := os.Open("facebookSecond.png")
		reader2, err2 := os.Open("facebookFirst.png")
		reader3, err3 := os.Open("proxyFacebookSecond.png")
		reader4, err4 := os.Open("proxyFacebookFirst.png")
		reader5, err5 := os.Open("proxyFacebookHTML.html")
		reader6, err6 := os.Open("facebookHTML.html")

		defer reader.Close()
		defer reader2.Close()
		defer reader3.Close()
		defer reader4.Close()
		defer reader5.Close()
		defer reader6.Close()
		if err != nil || err2 != nil || err3 != nil ||
			err4 != nil || err5 != nil || err6 != nil {
			slog.Error("Could not load error images",
				slog.Any("err", err),
				slog.Any("err2", err2),
				slog.Any("err3", err3),
				slog.Any("err4", err4),
				slog.Any("err5", err5),
				slog.Any("err6", err6),
			)
		}
		Discord.ChannelFileSend(ChannelID, "second.png", reader)
		Discord.ChannelFileSend(ChannelID, "first.png", reader2)
		Discord.ChannelFileSend(ChannelID, "proxySecond.png", reader3)
		Discord.ChannelFileSend(ChannelID, "proxyFirst.png", reader4)
		Discord.ChannelFileSend(ChannelID, "proxyHTML.html", reader5)
		Discord.ChannelFileSend(ChannelID, "facebook.html", reader6)
	}
	if strings.Contains(err.Error(), "Ebay") {
		reader, err := os.Open("ebaySecond.png")
		reader2, err2 := os.Open("ebayFirst.png")
		defer reader.Close()
		defer reader2.Close()
		if err != nil || err2 != nil {
			slog.Error("Could not load error images", slog.Any("Error", err),
				slog.Any("Error", err2))
		}
		Discord.ChannelFileSend(ChannelID, "second.png", reader)
		Discord.ChannelFileSend(ChannelID, "first.png", reader2)
	}
	if strings.Contains(err.Error(), "default") {
		reader, err := os.Open("failoverSS.png")
		reader2, err2 := os.Open("failoverHTML.html")
		reader3, err3 := os.Open("collyHTML.html")
		reader4, err4 := os.Open("proxyFailoverSS.png")
		reader5, err5 := os.Open("proxyFailoverHTML.html")
		defer reader.Close()
		defer reader2.Close()
		defer reader3.Close()
		defer reader4.Close()
		defer reader5.Close()
		if err != nil || err2 != nil || err3 != nil ||
			err4 != nil || err5 != nil {
			slog.Error("Could not send error image",
				slog.Any("Error", err),
				slog.Any("Error HTML File", err2),
				slog.Any("err3", err3),
				slog.Any("err4", err4),
				slog.Any("err5", err5),
			)
		}
		Discord.ChannelFileSend(ChannelID, "collyHTML.html", reader3)
		Discord.ChannelFileSend(ChannelID, "failoverSS.png", reader)
		Discord.ChannelFileSend(ChannelID, "failoverHTML.html", reader2)
		Discord.ChannelFileSend(ChannelID, "proxyFailoverSS.png", reader4)
		Discord.ChannelFileSend(ChannelID, "proxyFailoverHTML.html", reader5)
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &discordgo.MessageEmbed{
		Title:  "Error",
		Fields: Fields,
		Color:  red,
	})
}

// SendGraphPng sends a price history graph as a file to a Discord channel.
//
// Parameters:
//   - discord: the Discord session
//   - ChannelID: the Discord channel ID
func SendGraphPng(discord *discordgo.Session, ChannelID string) {
	reader, err := os.Open("my-chart.png")
	if err != nil {
		slog.Error("Could not load graph image", slog.Any("Error", err))
	}
	discord.ChannelFileSend(ChannelID, "my-chart.png", reader)
}

// autoComplete handles Discord autocomplete interactions for command options.
// t specifies the field type: 0=item name, 1=url, 2=css selector, 3=alternate tracking queries, 4=exclusion queries.
//
// Parameters:
//   - Name: the current input value
//   - t: the type of autocomplete (0=name, 1=url, 2=css, 3=alt tracking, 4=exclusion)
//   - i: the Discord interaction
//   - discord: the Discord session
func autoComplete(Name string, t int, i *discordgo.InteractionCreate, discord *discordgo.Session) {
	var choices []*discordgo.ApplicationCommandOptionChoice
	var items []string
	switch t {
	case 0:
		items = database.FuzzyMatchName(Name, i.ChannelID)
	case 1:
		items = database.AutoCompleteURL(Name, i.ChannelID)
	case 3:
		items = database.AutoCompleteAlternateTrackingQueries(Name, i.ChannelID)
	case 4:
		items = database.AutoCompleteTrackingExclusionQueries(Name, i.ChannelID)
	}

	if len(items) != 0 {
		for index, item := range items {
			var choice discordgo.ApplicationCommandOptionChoice
			if len(item) > 100 {
				choice = discordgo.ApplicationCommandOptionChoice{
					Name:  "item too long" + item[8:20],
					Value: "placeholder",
				}
			} else {
				choice = discordgo.ApplicationCommandOptionChoice{
					Name:  item,
					Value: item,
				}
			}
			// if its a url or alternate tracking query do by index instead
			if t == 1 || t == 3 {
				choice.Value = index
			}
			choices = append(choices, &choice)
		}
		err := discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: choices,
			},
		})
		if err != nil {
			slog.Error("auto complete error", slog.Any("Error", err))
		}
	} else {
		err := discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "No Items Found",
						Value: "No Items Found",
					},
				},
			},
		})
		if err != nil {
			slog.Error("auto complete error", slog.Any("Error", err))
		}
	}
}

// autoCompleteQuerySelector handles auto compelte for discord handler
// when user inputs HTMLQuery values
func autoCompleteQuerySelector(i *discordgo.InteractionCreate, discord *discordgo.Session) {
	items := database.AutoCompleteQuery()
	var choices []*discordgo.ApplicationCommandOptionChoice
	for name, query := range items {
		choice := discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: query,
		}
		choices = append(choices, &choice)
	}
	err := discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
	if err != nil {
		slog.Error("auto complete error for query select", slog.Any("Error", err))
	}
}

// EbayListingPriceChangeAlert sends an alert when a second-hand listing price changes.
//
// Parameters:
//   - newListing: the updated listing information
//   - oldPrice: the previous price
//   - ChannelID: the Discord channel ID
//   - aggregate: the aggregate report for pricing context
func EbayListingPriceChangeAlert(newListing *types.EbayListing,
	oldPrice int, ChannelID string, aggregate *database.AggregateReport,
) {
	var colorCode int
	if oldPrice < newListing.Price {
		if newListing.Price > aggregate.AveragePrice {
			colorCode = extremeRed // price higher and higher than average
		} else {
			colorCode = aqua // price higher but lower than average
		}
	} else {
		if newListing.Price > aggregate.AveragePrice {
			colorCode = orange // price lowered but bigger than average
		} else {
			colorCode = razerGreen // price lowered and lower than average
		}
	}
	newFields := formatSecondHandField(newListing, "New Price", true, true)
	oldP := &types.EbayListing{
		Price: oldPrice,
		Title: newListing.Title,
	}
	oldFields := formatSecondHandField(oldP, "Old Price", false, false)
	em := discordgo.MessageEmbed{
		Title:  "Second Hand Listing Price Change For " + newListing.ItemName,
		Color:  colorCode,
		Fields: append(newFields, oldFields...),
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

// NewEbayListingAlert sends an alert when a new second-hand listing is found.
//
// Parameters:
//   - newListing: the new listing information
//   - ChannelID: the Discord channel ID
//   - aggregate: the aggregate report for pricing context
func NewEbayListingAlert(newListing *types.EbayListing,
	ChannelID string, aggregate *database.AggregateReport,
) {
	color := pink
	if newListing.Price < aggregate.AveragePrice {
		color = green
	}
	fields := formatSecondHandField(newListing, "Price", true, false)
	em := discordgo.MessageEmbed{
		Title:  "New Second Hand Listing Found For " + newListing.ItemName,
		Color:  color,
		Fields: fields,
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

// NewBidAlert sends an alert when a new bid is found for an item.
//
// Parameters:
//   - newListing: the new bid information
//   - ChannelID: the Discord channel ID
//   - aggregate: the aggregate report for pricing context
func NewBidAlert(newListing *types.EbayBids,
	ChannelID string, aggregate *database.AggregateReport,
) {
	color := darkGold
	if newListing.Price < aggregate.AveragePrice {
		color = blue
	}
	fields := formatBidField(newListing, false)
	em := discordgo.MessageEmbed{
		Title:  "New Bid For " + newListing.ItemName,
		Color:  color,
		Fields: fields,
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

// BidPriceChangeAlert sends an alert when a bid price changes.
//
// Parameters:
//   - newListing: the updated bid information
//   - oldListing: the previous bid information
//   - ChannelID: the Discord channel ID
//   - aggregate: the aggregate report for pricing context
func BidPriceChangeAlert(newListing *types.EbayBids,
	oldListing *types.EbayBids, ChannelID string, aggregate *database.AggregateReport,
) {
	var colorCode int
	if oldListing.Price < newListing.Price {
		if newListing.Price > aggregate.AveragePrice {
			colorCode = darkGold // price higher and higher than average
		} else {
			colorCode = blue // price higher but lower than average
		}
	} else {
		if newListing.Price > aggregate.AveragePrice {
			colorCode = orange // price lowered but bigger than average
		} else {
			colorCode = razerGreen // price lowered and lower than average
		}
	}
	newFields := formatBidField(newListing, false)
	oldFields := formatBidField(oldListing, true)
	em := discordgo.MessageEmbed{
		Title:  "Bid Update for " + newListing.ItemName,
		Color:  colorCode,
		Fields: append(newFields, oldFields...),
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

// sendWelcomeMessage sends the welcome message when the bot joins a new server.
func sendWelcomeMessage(discord *discordgo.Session, channelID string) {
	message := "**Welcome to PriceTracker!** 🎉\n\n" +
		"**Getting Started:**\n" +
		"1. Run `/setup` in this channel to configure your location and marketplace settings\n" +
		"2. Use `/add` to start tracking items\n" +
		"3. Check `/help` for all available commands\n\n" +
		"For more information, visit: https://github.com/niknak1379/discord-price-tracker"

	_, err := discord.ChannelMessageSend(channelID, message)
	if err != nil {
		slog.Error("could not send welcome message",
			slog.String("ChannelID", channelID),
			slog.Any("Error", err))
	}
}

// customAcknowledge sends an immediate acknowledgment response to Discord.
// Used for commands that may take longer than 3 seconds to complete.
// for functions that will take too long(more than the 15 min resposne time
// required)
//
// Parameters:
//   - discord: the Discord session
//   - i: the Discord interaction
//
// Returns any error encountered.
func customAcknowledge(discord *discordgo.Session, i *discordgo.InteractionCreate) error {
	em := discordgo.MessageEmbed{}
	data := i.ApplicationCommandData().Options
	Name := discordgo.MessageEmbedField{
		Name: i.ApplicationCommandData().Name,
	}
	em.Fields = append(em.Fields, &Name)
	for _, option := range data {
		field := discordgo.MessageEmbedField{
			Name:  option.Name,
			Value: fmt.Sprintf("%v", option.Value),
		}
		em.Fields = append(em.Fields, &field)
	}
	err := discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{&em},
		},
	})
	return err
}
