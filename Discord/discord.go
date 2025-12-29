package discord

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	charts "priceTracker/Charts"
	database "priceTracker/Database"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

var (
	BotToken    string
	Discord     *discordgo.Session
	commandList = []*discordgo.ApplicationCommand{
		{
			Name:        "add",
			Description: "Add new Price Tracker",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Add item name",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:        "uri",
					Description: "Add Scrapping URI",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:         "html_tag",
					Description:  "Add Scrapping HTML Tag",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "get",
			Description: "Add all links for the item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "Add item name",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "list",
			Description: "get all items",
		},
		{
			Name:        "remove",
			Description: "remove item completely",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "Add item name",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "edit",
			Description: "Edit a currently Existing Tracker",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "add",
					Description: "add new pair of tracking URI and HTML",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:         "name",
							Description:  "Add item name",
							Type:         discordgo.ApplicationCommandOptionString,
							Required:     true,
							Autocomplete: true,
						},
						{
							Name:        "uri",
							Description: "Add Scrapping URI",
							Type:        discordgo.ApplicationCommandOptionString,
							Required:    true,
						},
						{
							Name:         "html_tag",
							Description:  "Add Scrapping HTML Tag",
							Type:         discordgo.ApplicationCommandOptionString,
							Required:     true,
							Autocomplete: true,
						},
					},
				},
				{
					Name:        "remove",
					Description: "remove pair of tracking URI and HTML",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Options: []*discordgo.ApplicationCommandOption{
						{
							Name:         "name",
							Description:  "Add item name",
							Type:         discordgo.ApplicationCommandOptionString,
							Required:     true,
							Autocomplete: true,
						},
						{
							Name:         "uri",
							Description:  "Add Scrapping URI",
							Type:         discordgo.ApplicationCommandOptionString,
							Required:     true,
							Autocomplete: true,
						},
					},
				},
			},
		},
		{
			Name:        "graph",
			Description: "graph price of item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "Add item name",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "months",
					Description: "how long of the history to graph",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
			},
		},
	}
)

var commandHandler = map[string]func(discord *discordgo.Session, i *discordgo.InteractionCreate){
	"add": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoCompleteQuerySelector(i, discord)
		default:
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})

			// get command inputs from discord
			options := i.ApplicationCommandData().Options
			// 0 is item name, 1 is uri, 2 is htmlqueryselector
			content := ""
			var em *discordgo.MessageEmbed
			// add tracker to database
			addRes, err := database.AddItem(options[0].StringValue(), options[1].StringValue(), options[2].StringValue())
			if err != nil {
				content = fmt.Sprint(err)
				discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: content,
				})
				return
			} else {
				em = setEmbed(addRes)
			}
			// set up response to discord client
			discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: content,
				Embeds:  []*discordgo.MessageEmbed{em},
			})
		}
	},

	"get": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		// 0 is item name, 1 is uri, 2 is htmlqueryselector
		content := ""
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
		default:
			// add tracker to database
			getRes, err := database.GetItem(options[0].StringValue())
			var embedArr []*discordgo.MessageEmbed
			if err != nil {
				content = err.Error()
			} else {
				em := setEmbed(getRes)
				embedArr = append(embedArr, em)
			}

			// set up response to discord client
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: content,
					Embeds:  embedArr,
				},
			})
		}
	},
	"list": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// add tracker to database
		getRes := database.GetAllItems()
		// returnstr, _ := json.Marshal(getRes)
		var embedArr []*discordgo.MessageEmbed
		for _, Item := range getRes {
			em := setEmbed(*Item)
			embedArr = append(embedArr, em)
		}

		// set up response to discord client
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: embedArr,
			},
		})
	},
	"remove": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
		default:
			// remove tracker to database
			deleteRes := database.RemoveItem(options[0].StringValue())

			// set up response to discord client
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Delted %d Rows in the DB", (deleteRes)),
				},
			})

		}
	},
	// this is hella unreadable refractor to make it look better
	"edit": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

		// handle auto correct requests for the different fields
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			logger.Info("auto complete interaction coming in", slog.Any("option", options))
			switch {
			case options[0].Options[0].Focused:
				autoComplete(options[0].Options[0].StringValue(), 0, i, discord)
			case options[0].Options[1].Focused:
				autoComplete(options[0].Options[0].StringValue(), 1, i, discord)
			case options[0].Options[2].Focused:
				autoCompleteQuerySelector(i, discord)
			}
		default:
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			content := ""
			embeds := []*discordgo.MessageEmbed{}
			// get option values
			name := options[0].Options[0].StringValue()
			uri := options[0].Options[1].StringValue()

			// handle add and remove subcommands
			switch options[0].Name {
			case "add":
				htmlQuery := options[0].Options[2].StringValue()

				// database reutrns a price struct, setpricefield formats the returned price
				// and adds it to the message embeds
				res, p, err := database.AddTrackingInfo(name, uri, htmlQuery)
				priceField := setPriceField(p, "Newly Added Tracker")

				// add price tracking info
				em := setEmbed(res)
				em.Fields = append(em.Fields, priceField...)
				if err != nil {
					content = err.Error()
				} else {
					embeds = append(embeds, em)
				}

			case "remove":
				res, err := database.RemoveTrackingInfo(name, uri)
				em := setEmbed(res)
				if err != nil {
					content = err.Error()
				} else {
					embeds = append(embeds, em)
				}
			}

			discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: content,
				Embeds:  embeds,
			})

		}
	},
	"graph": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options

		// handle autocomplete for name and normal request
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
		default:
			// set up response to discord client
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// get command inputs from discord
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			logger.Info("options", slog.Any("optoin", options))
			err := charts.PriceHistoryChart(options[0].StringValue(), int(options[1].IntValue()))
			if err != nil {
				log.Print(err)
				discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: fmt.Sprint(err),
				})
			} else {
				reader, err := os.Open("my-chart.png")
				if err != nil {
					log.Fatal(err)
				}
				File := discordgo.File{
					Name:        "chart.png",
					ContentType: "Image",
					Reader:      reader,
				}
				_, err = discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Files: []*discordgo.File{&File},
				})
				if err != nil {
					fmt.Printf("Error sending follow-up message: %v\n", err)
				}
			}
		}
	},
}

func Run(ctx context.Context) {
	// create a session
	var err error
	Discord, err = discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Panic("could not connect to discord client", err)
	}

	Discord.AddHandler(ready)

	// open session
	Discord.Open()

	Discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandler[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commandList))
	for index, command := range commandList {
		cmd, err := Discord.ApplicationCommandCreate(Discord.State.User.ID, "", command)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", command.Name, err)
		}
		registeredCommands[index] = cmd
	}
	log.Println("all commands added")

	// keep the bot open until sigint is recieved from ctx in main
	<-ctx.Done()
	log.Println("Removing commands...")
	registeredCommands, err = Discord.ApplicationCommands(Discord.State.User.ID, "")
	if err != nil {
		log.Panicf("Cannot get application registered command list")
	}
	for _, v := range registeredCommands {
		err = Discord.ApplicationCommandDelete(Discord.State.User.ID, "", v.ID)
		if err != nil {
			log.Printf("Cannot delete '%v' command: %v", v.Name, err)
		}
	}
	Discord.Close()
	log.Println("shut down discord")
}

func ready(discord *discordgo.Session, ready *discordgo.Ready) {
	fmt.Println("Logged in")
	discord.UpdateGameStatus(1, "stonks")
}

func LowestPriceAlert(discord *discordgo.Session, itemName string, newPrice int, oldPrice int, URL string) {
	content := fmt.Sprintf("New Price Alert!!!!\nItem %s has hit its lowest price of %d "+
		"from previous lowest of %d with the following url \n%s",
		itemName, newPrice, oldPrice, URL)
	discord.ChannelMessageSend(os.Getenv("CHANNEL_ID"), content)
}

func CrawlErrorAlert(discord *discordgo.Session, itemName string, URL string, err error) {
	content := fmt.Sprintf("Crawler could not find price for %s in url %s, with error %s investigate logs for further information",
		itemName, URL, err.Error())
	log.Printf("Crawler could not find price for %s in url %s, with error %s investigate logs for further information",
		itemName, URL, err.Error())
	discord.ChannelMessageSend(os.Getenv("CHANNEL_ID"), content)
}

func SendGraphPng(discord *discordgo.Session) {
	// content := fmt.Sprintf("Chart for Product named %s for the last %d months", productName, months)
	reader, err := os.Open("my-chart.png")
	if err != nil {
		log.Fatal(err)
	}
	discord.ChannelFileSend(os.Getenv("CHANNEL_ID"), "my-chart.png", reader)
}

func setEmbed(Item database.Item) *discordgo.MessageEmbed {
	var fields []*discordgo.MessageEmbedField
	// set up trackerArr infromation
	field := discordgo.MessageEmbedField{
		Name:   embedSeparatorFormatter("Tracking URL", 43),
		Value:  embedSeparatorFormatter("CSS Selector", 44),
		Inline: false,
	}
	fields = append(fields, &field)
	for _, tracker := range Item.TrackingList {
		field := discordgo.MessageEmbedField{
			Name:   tracker.URI,
			Value:  tracker.HtmlQuery,
			Inline: false,
		}
		separatorField := discordgo.MessageEmbedField{
			Name:   embedSeparatorFormatter("", 45),
			Value:  "",
			Inline: false,
		}
		fields = append(fields, &field, &separatorField)
	}

	// set up current price information
	priceFields := setPriceField(Item.CurrentLowestPrice, "Current")
	lowestPriceField := setPriceField(Item.LowestPrice, "Historically Lowest")
	fields = append(fields, priceFields...)
	fields = append(fields, lowestPriceField...)
	em := discordgo.MessageEmbed{
		Title: Item.Name,
		Image: &discordgo.MessageEmbedImage{
			URL: Item.ImgURL,
		},
		Fields: fields,
		Type:   discordgo.EmbedTypeRich,
	}
	fmt.Println(Item.ImgURL)
	return &em
}

func setPriceField(p database.Price, message string) []*discordgo.MessageEmbedField {
	/* separatorField := discordgo.MessageEmbedField{
		Name: "<------------------------------------------->",
		Value: "",
		Inline: false,
	} */
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
		Value:  p.Url,
		Inline: false,
	}
	var fields []*discordgo.MessageEmbedField
	fields = append(fields, &priceField, &urlField)
	return fields
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
