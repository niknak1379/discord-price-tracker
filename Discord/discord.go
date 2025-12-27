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
					Name:        "html_tag",
					Description: "Add Scrapping HTML Tag",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "get",
			Description: "Add all links for the item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Add item name",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
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
					Name:        "name",
					Description: "Add item name",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
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
							Name:        "html_tag",
							Description: "Add Scrapping HTML Tag",
							Type:        discordgo.ApplicationCommandOptionString,
							Required:    true,
						},
					},
				},
				{
					Name:        "remove",
					Description: "remove pair of tracking URI and HTML",
					Type:        discordgo.ApplicationCommandOptionSubCommand,
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
					},
				},
			},
		},
		{
			Name:        "graph",
			Description: "graph price of item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "name",
					Description: "Add item name",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
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
		} else {
			em = setEmbed(addRes)
		}

		// set up response to discord client
		discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Content: content,
			Embeds:  []*discordgo.MessageEmbed{em},
		})
	},

	"get": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		// 0 is item name, 1 is uri, 2 is htmlqueryselector
		content := ""

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
		// get command inputs from discord
		options := i.ApplicationCommandData().Options

		// add tracker to database
		deleteRes := database.RemoveItem(options[0].StringValue())

		// set up response to discord client
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: fmt.Sprint(deleteRes),
			},
		})
	},
	"edit": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		options := i.ApplicationCommandData().Options
		content := ""
		embeds := []*discordgo.MessageEmbed{}
		// get option values
		name := options[0].Options[0].StringValue()
		uri := options[0].Options[1].StringValue()

		// As you can see, names of subcommands (nested, top-level)
		// and subcommand groups are provided through the arguments.
		switch options[0].Name {
		case "add":
			htmlQuery := options[0].Options[2].StringValue()
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
	},
	"graph": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// set up response to discord client
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
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
	},
}

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Error message", e)
	}
}

func Run(ctx context.Context) {
	// create a session
	var err error
	Discord, err = discordgo.New("Bot " + BotToken)
	checkNilErr(err)

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
	<-ctx.Done()
	Discord.Close()
	log.Println("Removing commands...")
	registeredCommands, err = Discord.ApplicationCommands(Discord.State.User.ID, "")
	if err != nil {
		log.Panicf("Cannot get application registered command list")
	}
	for _, v := range registeredCommands {
		err = Discord.ApplicationCommandDelete(Discord.State.User.ID, "", v.ID)
		if err != nil {
			log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
		}
	}
	log.Println("Gracefully shutting down.")
}

func ready(discord *discordgo.Session, ready *discordgo.Ready) {
	fmt.Println("Logged in")
	discord.UpdateGameStatus(1, "stonks")
}

func LowestPriceAlert(discord *discordgo.Session, itemName string, newPrice int, oldPrice int, URL string) {
	content := fmt.Sprintf("New Price Alert!!!!\nItem %s has hit its lowest price of %d "+
		"from previous lowest of %d with the following url \n%s",
		itemName, newPrice, oldPrice, URL)
	discord.ChannelMessageSend("803818389755265075", content)
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
		Title:  Item.Name,
		Fields: fields,
		Type:   discordgo.EmbedTypeRich,
	}
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
