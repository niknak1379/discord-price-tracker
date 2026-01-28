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

func ready(discord *discordgo.Session, ready *discordgo.Ready) {
	slog.Info("Discord Logged in")
	discord.UpdateGameStatus(1, "stonks")
}

func PriceChangeAlert(itemName string, newPrice int, oldPrice database.Price, URL string, ChannelID string) {
	var color int
	if newPrice > oldPrice.Price {
		color = 16776960
	} else {
		color = 2067276
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
		Title:       "Price Update:",
		Description: itemName,
		Color:       color,
		URL:         URL,
		Fields:      Fields,
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

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
	} else if strings.Contains(err.Error(), "Ebay") {
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
	} else {
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
		Color:  10038562, // red
	})
}

func SendGraphPng(discord *discordgo.Session, ChannelID string) {
	reader, err := os.Open("my-chart.png")
	if err != nil {
		slog.Error("Could not load graph image", slog.Any("Error", err))
	}
	discord.ChannelFileSend(ChannelID, "my-chart.png", reader)
}

func autoComplete(Name string, t int, i *discordgo.InteractionCreate, discord *discordgo.Session) {
	var choices []*discordgo.ApplicationCommandOptionChoice
	var items []string
	// t int value 0 maps to name type, 1 to url type, 2 to css
	switch t {
	case 0:
		items = database.FuzzyMatchName(Name, i.ChannelID)
	case 1:
		items = database.AutoCompleteURL(Name, i.ChannelID)
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
			// if its a url do by index instead
			if t == 1 {
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

func EbayListingPriceChangeAlert(newListing *types.EbayListing, oldPrice int, ChannelID string) {
	colorCode := 1752220 // aqua
	if oldPrice < newListing.Price {
		colorCode = 12745742 // dark gold
	}
	newFields := formatSecondHandField(newListing, "New Price", true)
	newListing.Price = oldPrice
	oldFields := formatSecondHandField(newListing, "Old Price", false)
	em := discordgo.MessageEmbed{
		Title:  "Second Hand Listing Price Change For " + newListing.ItemName,
		Color:  colorCode,
		URL:    newListing.URL,
		Fields: append(oldFields, newFields...),
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

func NewEbayListingAlert(newListing *types.EbayListing, ChannelID string) {
	fields := formatSecondHandField(newListing, "Price", true)
	em := discordgo.MessageEmbed{
		Title:  "New Second Hand Listing Found For " + newListing.ItemName,
		Color:  15277667, // pink
		URL:    newListing.URL,
		Fields: fields,
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

// for functions that will take too long(more than the 15 min resposne time
// required)
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
