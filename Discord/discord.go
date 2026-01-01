package discord

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	charts "priceTracker/Charts"
	database "priceTracker/Database"
	"syscall"

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
			Name:        "restart",
			Description: "Saves Progress and Stops the Bot",
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
			addRes, err := database.AddItem(options[0].StringValue(), options[1].StringValue(), options[2].StringValue(), i.ChannelID)
			if err != nil {
				content = fmt.Sprint(err)
				discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: content,
				})
				return
			} else {
				em = setEmbed(&addRes)
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
			getRes, err := database.GetItem(options[0].StringValue(), i.ChannelID)
			var embedArr []*discordgo.MessageEmbed
			if err != nil {
				content = err.Error()
			} else {
				em := setEmbed(&getRes)
				embedArr = append(embedArr, em)
			}

			// set up response to discord client
			err = discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: content,
					Embeds:  embedArr,
				},
			})
			fmt.Println(err)
		}
	},
	"edit_name": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options

		content := ""
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
		default:
			// add tracker to database
			getRes, err := database.EditName(options[0].StringValue(), options[1].StringValue(), i.ChannelID)
			var embedArr []*discordgo.MessageEmbed
			if err != nil {
				content = err.Error()
			} else {
				em := setEmbed(&getRes)
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
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		getRes := database.GetAllItems(i.ChannelID)
		// returnstr, _ := json.Marshal(getRes)
		
		for _, Item := range getRes {
			em := setEmbed(Item)
			_, err := discord.ChannelMessageSendEmbed(i.ChannelID, em)
			fmt.Println("error from list",err)
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
				em.Fields = append(em.Fields, priceField...)
				if err != nil {
					content = err.Error()
				} else {
					embeds = append(embeds, em)
				}

			case "remove":
				res, err := database.RemoveTrackingInfo(name, uri, i.ChannelID)
				em := setEmbed(&res)
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
			err := charts.PriceHistoryChart(options[0].StringValue(), int(options[1].IntValue()), i.ChannelID)
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
