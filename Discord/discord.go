package discord

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"time"

	charts "priceTracker/Charts"
	database "priceTracker/Database"

	"github.com/bwmarrin/discordgo"
)

var (
	BotToken    string
	Discord     *discordgo.Session
	commandList = []*discordgo.ApplicationCommand{
		{
			Name:        "setup",
			Description: "create new tracking table",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "location",
					Description: "set marketplace location, with format -City Name, State- ",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:        "marketplace-location-code",
					Description: "Location Code market place uses to represent ur region",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
				{
					Name:        "distance",
					Description: "set marketplace max distance",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
			},
		},
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
				{
					Name:         "timer",
					Description:  "interval between scrapes, in hours",
					Type:         discordgo.ApplicationCommandOptionInteger,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "type",
					Description: "Item Type",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Tech",
							Value: "Tech",
						},
						{
							Name:  "Clothes",
							Value: "Clothes",
						},
					},
				},
			},
		},
		{
			Name:        "suppress",
			Description: "Suppress notifications for this item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "Add item name",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "suppress",
					Description: "bool, wether to suppress or not",
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Required:    true,
				},
			},
		},
		{
			Name:        "edit_timer",
			Description: "Suppress notifications for this item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "Add item name",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "timer",
					Description: "New timer",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
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
			Name:        "edit_name",
			Description: "Edit Item Name(Used for Ebay queries",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "old_name",
					Description:  "name of item to be changed",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "new_name",
					Description: "name of item to be changed",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "edit_tracking",
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
		{
			Name:        "graph-compare",
			Description: "graph price of items",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name1",
					Description:  "Add item name",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:         "name2",
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
		{
			Name:        "aggregate",
			Description: "Get Aggregate Data for the Used Listings of the Item",
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
					Description: "how long of the history to aggregate",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
				{
					Name:        "ending_month",
					Description: "how many months ago the ending point of the aggregation should be",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
			},
		},
		{
			Name:        "restart",
			Description: "Saves Progress and Stops the Bot",
		},
	}
)

var commandHandler = map[string]func(discord *discordgo.Session, i *discordgo.InteractionCreate){
	"setup": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options

		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		// add tracker to database
		err := database.CreateChannelItemTableIfMissing(i.ChannelID,
			options[0].StringValue(),
			options[1].StringValue(),
			int(options[1].IntValue()))
		if err != nil {
			content := err.Error()
			discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title: "priceTracker",
						Color: 10038562, // red
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:  "Setup unSuccessful",
								Value: content,
							},
						},
					},
				},
			})
		} else {
			discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title: "priceTracker",
						Color: 10181046, // purple
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:  "Setup Successful",
								Value: "",
							},
						},
					},
				},
			})
		}
	},
	"add": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoCompleteQuerySelector(i, discord)
		default:
			err := customAcknowledge(discord, i)
			if err != nil {
				slog.Error("ack error", slog.Any("error value", err))
			}
			// get command inputs from discord
			options := i.ApplicationCommandData().Options
			// 0 is item name, 1 is uri, 2 is htmlqueryselector
			content := ""
			var em []*discordgo.MessageEmbed
			// add tracker to database
			addRes, err := database.AddItem(options[0].StringValue(),
				options[1].StringValue(), options[2].StringValue(),
				options[4].StringValue(), int(options[3].IntValue()), database.ChannelMap[i.ChannelID])
			if err != nil {
				content = fmt.Sprint(err)
				discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "Error adding item" + content,
				})
				CrawlErrorAlert(options[0].StringValue(), options[1].StringValue(), err, i.ChannelID)
				return
			} else {
				em = setEmbed(&addRes)
			}
			// set up response to discord client
			_, err = discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: content,
				Embeds:  em,
			})
			if err != nil {
				for _, embed := range em {
					discord.ChannelMessageSendEmbed(i.ChannelID, embed)
				}
			}
		}
	},
	"edit_timer": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
		default:
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// add tracker to database
			err := database.EditTimer(options[0].StringValue(), int(options[1].IntValue()), i.ChannelID)
			content := ""
			if err != nil {
				content = err.Error()
			} else {
				content = fmt.Sprintf("Price Update Notification Timer: %d Hours", int(options[1].IntValue()))
			}
			discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: content,
			})
		}
	},
	"suppress": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
		default:
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// add tracker to database
			err := database.EditSuppress(options[0].StringValue(), options[1].BoolValue(), i.ChannelID)
			content := ""
			if err != nil {
				content = err.Error()
			} else {
				content = fmt.Sprintf("Price Update Notification Status: %t", options[1].BoolValue())
			}
			discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: content,
			})
		}
	},
	"get": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		// 0 is item name, 1 is uri, 2 is htmlqueryselector
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
		default:
			err := customAcknowledge(discord, i)
			if err != nil {
				slog.Error("ack error", slog.Any("error value", err))
			}
			getRes, err := database.GetItem(options[0].StringValue(), i.ChannelID)
			if err != nil {
				content := err.Error()
				discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
					Content: content,
				})
			} else {
				em := setEmbed(&getRes)

				// set up response to discord client
				for _, embed := range em {
					_, err = discord.ChannelMessageSendEmbed(i.ChannelID, embed)
					if err != nil {
						slog.Error("failed to send embed",
							slog.Any("Embed", embed),
							slog.Any("value", err),
						)
					}
				}
			}
		}
	},
	"edit_name": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options

		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
			return
		case discordgo.InteractionApplicationCommand:
			getRes, err := database.EditName(options[0].StringValue(), options[1].StringValue(), i.ChannelID)
			var embedArr []*discordgo.MessageEmbed
			var content string

			if err != nil {
				content = "Error: " + err.Error()
				discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Data: &discordgo.InteractionResponseData{
						Content: content,
					},
				})
			} else {
				em := setEmbed(&getRes)
				embedArr = append(embedArr, em...)
			}

			for _, embed := range embedArr {
				_, err = discord.ChannelMessageSendEmbed(i.ChannelID, embed)
				if err != nil {
					slog.Error("failed to send embed",
						slog.Any("Embed", embed),
						slog.Any("value", err),
					)
				}
			}
		}
	},
	"list": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		err := customAcknowledge(discord, i)
		if err != nil {
			slog.Error("ack error", slog.Any("error value", err))
		}
		getRes := database.GetAllItems(i.ChannelID)
		// returnstr, _ := json.Marshal(getRes)

		for _, Item := range getRes {
			em := setEmbed(Item)
			for _, embed := range em {
				discord.ChannelMessageSendEmbed(i.ChannelID, embed)
			}
		}
		if len(getRes) == 0 {
			_, err := discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: "No Items Are Being Tracked in This Channel",
			})
			if err != nil {
				slog.Error("Could not send response",
					slog.Any("value", err),
				)
			}
		}
	},
	"remove": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
		default:
			// remove tracker to database
			deleteRes := database.RemoveItem(options[0].StringValue(), i.ChannelID)

			// set up response to discord client
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Deleted Rows in the DB: %d", (deleteRes)),
				},
			})

		}
	},
	// this is hella unreadable refractor to make it look better
	"edit_tracking": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
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
				res, p, err := database.AddTrackingInfo(name, uri, htmlQuery, i.ChannelID)
				priceField := setPriceField(&p, "Newly Added Tracker")

				// add price tracking info
				em := setEmbed(&res)
				em[len(em)-1].Fields = append(em[len(em)-1].Fields, priceField...)
				if err != nil {
					content = err.Error()
				} else {
					embeds = append(embeds, em...)
				}

			case "remove":
				res, err := database.RemoveTrackingInfo(name, uri, i.ChannelID)
				em := setEmbed(&res)
				if err != nil {
					content = err.Error()
				} else {
					embeds = append(embeds, em...)
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
			err := charts.PriceHistoryChart([]string{options[0].StringValue()}, int(options[1].IntValue()), i.ChannelID)
			if err != nil {
				discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: fmt.Sprint(err),
				})
			} else {
				reader, err := os.Open("my-chart.png")
				if err != nil {
					slog.Error("Could not open file", slog.Any("Error", err))
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
					if err != nil {
						slog.Error("failed to send graph",
							slog.Any("value", err),
						)
					}
				}
			}
		}
	},
	"graph-compare": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		// handle autocomplete for name and normal request
		switch i.Type {

		case discordgo.InteractionApplicationCommandAutocomplete:
			switch {
			case options[0].Focused:
				autoComplete(options[0].StringValue(), 0, i, discord)
			case options[1].Focused:
				autoComplete(options[1].StringValue(), 0, i, discord)
			}

		default:
			// set up response to discord client
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// get command inputs from discord
			err := charts.PriceHistoryChart([]string{
				options[0].StringValue(),
				options[1].StringValue(),
			}, int(options[2].IntValue()), i.ChannelID)
			if err != nil {
				discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: fmt.Sprint(err),
				})
			} else {
				reader, err := os.Open("my-chart.png")
				if err != nil {
					slog.Error("Could not open file", slog.Any("Error", err))
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
					slog.Error("failed to send comparison graph")
				}
			}
		}
	},
	"aggregate": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
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
			//
			endDate := time.Now().AddDate(0, -1*int(options[2].IntValue()), 0)
			Aggregate, err := database.GenerateSecondHandPriceReport(
				options[0].StringValue(),
				endDate,
				int(options[1].IntValue())*30, i.ChannelID)
			content := ""
			var fields []*discordgo.MessageEmbedField
			if err != nil {
				content = err.Error()
			} else {
				startDate := endDate.AddDate(0, 0, -30*int(options[1].IntValue()))
				message := startDate.Format("2006-01-02") + " - " + endDate.Format("2006-01-02")
				fields = formatAggregateFields(Aggregate, message)
			}
			discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: content,
				Embeds: []*discordgo.MessageEmbed{
					{
						Fields: fields,
					},
				},
			})
		}
	},
	"restart": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "restarting server...",
			},
		})

		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	},
}

func channelDeleteHandler(discord *discordgo.Session, i *discordgo.ChannelDelete) {
	slog.Info("Channel being deleted with id: ", slog.String("ChannelID", i.Channel.ID))
	database.ChannelDeleteHandler(i.Channel.ID)
}

func Run(ctx context.Context) {
	// create a session
	var err error
	Discord, err = discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Panic("could not connect to discord client", err)
	}

	Discord.SyncEvents = false

	// sets bot label
	Discord.AddHandler(ready)
	Discord.AddHandler(channelDeleteHandler)

	// open session
	Discord.Open()

	Discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandler[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commandList))
	for index, command := range commandList {
		cmd, err := Discord.ApplicationCommandCreate(Discord.State.User.ID, "", command)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", command.Name, err)
		}
		registeredCommands[index] = cmd
	}
	slog.Info("all commands added")

	// keep the bot open until sigint is recieved from ctx in main
	<-ctx.Done()
	slog.Info("Removing commands...")
	registeredCommands, err = Discord.ApplicationCommands(Discord.State.User.ID, "")
	if err != nil {
		log.Panicf("Cannot get application registered command list")
	}
	var shutDownWG sync.WaitGroup
	for _, v := range registeredCommands {
		shutDownWG.Go(func() {
			Discord.ApplicationCommandDelete(Discord.State.User.ID, "", v.ID)
		})
	}
	shutDownWG.Wait()
	Discord.Close()
	slog.Info("Discord Shutdown")
}
