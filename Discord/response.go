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

func LowestPriceAlert(itemName string, newPrice int, oldPrice database.Price, URL string, ChannelID string) {
	oldPriceField := setPriceField(&oldPrice, "Previous Lowest")
	newPriceField := setPriceField(&database.Price{
		Price: newPrice,
		Url:   URL,
		Date:  time.Now(),
	}, "New Lowest")
	var Fields []*discordgo.MessageEmbedField
	Fields = append(Fields, oldPriceField...)
	Fields = append(Fields, newPriceField...)
	em := discordgo.MessageEmbed{
		Title:       "Price Update:",
		Description: itemName,
		Color:       2067276,
		URL:         URL,
		Fields:      Fields,
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

func CrawlErrorAlert(itemName string, URL string, err error, ChannelID string) {
	s := fmt.Sprintf("Crawler could not find price for %s in url %s, with error %s investigate logs for further information",
		itemName, URL, err.Error())
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
		defer reader.Close()
		defer reader2.Close()
		if err != nil || err2 != nil {
			slog.Error("Could not load error images", slog.Any("Error", err),
				slog.Any("Error", err2))
		}
		Discord.ChannelFileSend(ChannelID, "second.png", reader)
		Discord.ChannelFileSend(ChannelID, "first.png", reader2)
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
		defer reader.Close()
		if err != nil {
			slog.Error("Could not send error image", slog.Any("Error", err))
		}
		Discord.ChannelFileSend(ChannelID, "failoverSS.png", reader)

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
		for _, item := range items {
			if len(item) > 100 {
				choice := discordgo.ApplicationCommandOptionChoice{
					Name:  "item too long" + item[8:20],
					Value: "place holder",
				}
				choices = append(choices, &choice)
				continue
			}

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  item,
				Value: item,
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

func EbayListingPriceChangeAlert(newListing types.EbayListing, oldPrice int, ChannelID string) {
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

func NewEbayListingAlert(newListing types.EbayListing, ChannelID string) {
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
