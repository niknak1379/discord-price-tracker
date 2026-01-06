package discord

import (
	"fmt"
	"log"
	"math"
	"os"
	database "priceTracker/Database"
	types "priceTracker/Types"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

func ready(discord *discordgo.Session, ready *discordgo.Ready) {
	fmt.Println("Logged in")
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
	log.Printf("Crawler could not find price for %s in url %s, with error %s investigate logs for further information",
		itemName, URL, err.Error())
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
	// <--------------- send screenshots of failed crawl --------->
	fmt.Println("logging ebay existance", strings.Contains(err.Error(), "ebay"))
	if strings.Contains(err.Error(), "facebook") {
		reader, err := os.Open("second.png")
		reader2, err := os.Open("first.png")
		if err != nil {
			log.Println("could not send face book image", err)
		}
		Discord.ChannelFileSend(ChannelID, "second.png", reader)
		Discord.ChannelFileSend(ChannelID, "first.png", reader2)
	} else {
		reader, err := os.Open("failoverSS.png")
		if err != nil {
			log.Println("could not send ebay picture", err)
		}
		Discord.ChannelFileSend(ChannelID, "failoverSS.png", reader)

	}
	_, err = Discord.ChannelMessageSendEmbed(ChannelID, &discordgo.MessageEmbed{
		Title:  "Error",
		Fields: Fields,
		Color:  10038562, // red
	})
	if err != nil {
		fmt.Println(err.Error())
	}
}

func SendGraphPng(discord *discordgo.Session, ChannelID string) {
	// content := fmt.Sprintf("Chart for Product named %s for the last %d months", productName, months)
	reader, err := os.Open("my-chart.png")
	if err != nil {
		log.Fatal(err)
	}
	discord.ChannelFileSend(ChannelID, "my-chart.png", reader)
}

func autoComplete(Name string, t int, i *discordgo.InteractionCreate, discord *discordgo.Session) {
	var choices []*discordgo.ApplicationCommandOptionChoice
	fmt.Println("auto being run", Name)
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
				log.Println("printing from auto complete", item)
				choices = append(choices, &choice)
				continue
			}

			choice := discordgo.ApplicationCommandOptionChoice{
				Name:  item,
				Value: item,
			}
			log.Println("printing from auto complete", item, len(item))
			choices = append(choices, &choice)
		}
		err := discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{
				Choices: choices,
			},
		})
		if err != nil {
			log.Println(err)
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
			log.Println(err)
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
		log.Println("printing from auto complete query", name, query)
		choices = append(choices, &choice)
	}
	err := discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
	if err != nil {
		log.Println(err)
	}
}

func EbayListingPriceChangeAlert(newListing types.EbayListing, oldPrice int, ChannelID string) {
	colorCode := 1752220 // aqua
	if oldPrice < newListing.Price {
		fmt.Print("new price higher than old price")
		colorCode = 12745742 // dark gold
	}
	newFields := formatSecondHandField(newListing, "New Price")
	newListing.Price = oldPrice
	oldFields := formatSecondHandField(newListing, "Old Price")
	em := discordgo.MessageEmbed{
		Title:  "Second Hand Listing Price Change",
		Color:  colorCode,
		URL:    newListing.URL,
		Fields: append(oldFields, newFields...),
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}

func NewEbayListingAlert(newListing types.EbayListing, ChannelID string) {
	fields := formatSecondHandField(newListing, "Price")
	em := discordgo.MessageEmbed{
		Title:  "New Second Hand Listing Found",
		Color:  15277667, // pink
		URL:    newListing.URL,
		Fields: fields,
	}
	Discord.ChannelMessageSendEmbed(ChannelID, &em)
}
