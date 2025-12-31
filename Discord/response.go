package discord

import (
	"fmt"
	"log"
	"os"
	database "priceTracker/Database"
	types "priceTracker/Types"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func ready(discord *discordgo.Session, ready *discordgo.Ready) {
	fmt.Println("Logged in")
	discord.UpdateGameStatus(1, "stonks")
}

func LowestPriceAlert(itemName string, newPrice int, oldPrice int, URL string) {
	content := fmt.Sprintf("New Price Alert!!!!\nItem %s has hit its lowest price of %d "+
		"from previous lowest of %d with the following url \n%s",
		itemName, newPrice, oldPrice, URL)
	Discord.ChannelMessageSend(os.Getenv("CHANNEL_ID"), content)
}

func CrawlErrorAlert(itemName string, URL string, err error) {
	content := fmt.Sprintf("Crawler could not find price for %s in url %s, with error %s investigate logs for further information",
		itemName, URL, err.Error())
	log.Printf("Crawler could not find price for %s in url %s, with error %s investigate logs for further information",
		itemName, URL, err.Error())
	if strings.Contains(err.Error(), "ebay") {
		reader, err := os.Open("failoverSS.png")
		if err != nil {
			log.Println("could not send ebay picture", err)
		}
		Discord.ChannelFileSend(os.Getenv("CHANNEL_ID"), "my-chart.png", reader)
	} else if strings.Contains(err.Error(), "facebook") {
		reader, err := os.Open("second.png")
		if err != nil {
			log.Println("could not send face book image", err)
		}
		Discord.ChannelFileSend(os.Getenv("CHANNEL_ID"), "my-chart.png", reader)
	}
	Discord.ChannelMessageSend(os.Getenv("CHANNEL_ID"), content)
}

func SendGraphPng(discord *discordgo.Session) {
	// content := fmt.Sprintf("Chart for Product named %s for the last %d months", productName, months)
	reader, err := os.Open("my-chart.png")
	if err != nil {
		log.Fatal(err)
	}
	discord.ChannelFileSend(os.Getenv("CHANNEL_ID"), "my-chart.png", reader)
}

func autoComplete(Name string, t int, i *discordgo.InteractionCreate, discord *discordgo.Session) {
	var choices []*discordgo.ApplicationCommandOptionChoice
	fmt.Println("auto being run", Name)
	var items *[]string
	// t int value 0 maps to name type, 1 to url type, 2 to css
	switch t {
	case 0:
		items = database.FuzzyMatchName(Name)
	case 1:
		items = database.AutoCompleteURL(Name)
	}

	if len(*items) != 0 {
		for _, item := range *items {
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
	for name, query := range *items {
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

func EbayListingPriceChangeAlert(newListing types.EbayListing, oldPrice int) {
	content := fmt.Sprintf("Price update for %s Listing for %s with the price of $%d from the old price of $%d with the following url \n%s",
		newListing.Condition, newListing.Title, newListing.Price, oldPrice, newListing.URL)
	Discord.ChannelMessageSend(os.Getenv("CHANNEL_ID"), content)
}

func NewEbayListingAlert(newListing types.EbayListing) {
	content := fmt.Sprintf("New %s Listing for %s with the price of %d with the following url \n%s",
		newListing.Condition, newListing.Title, newListing.Price, newListing.URL)
	Discord.ChannelMessageSend(os.Getenv("CHANNEL_ID"), content)
}
