// Package discord implements the Discord bot interface for the price tracker.
// It handles command registration, user interactions, and sends price alerts to channels.
package discord

import (
	"context"
	"errors"
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
			Name:        "channel_info",
			Description: "get channel settings",
		},
		{
			Name:        "get_logs",
			Description: "returnes the latest html and pictures recorded",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "crawl",
					Description: "Item Type",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name:  "Facebook",
							Value: "facebook",
						},
						{
							Name:  "Ebay",
							Value: "Ebay",
						},
						{
							Name:  "Default",
							Value: "default",
						},
					},
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
					Name:        "timer",
					Description: "interval between scrapes, in hours",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
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
			Name:        "set_price",
			Description: "removes all trackers and manually sets price",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "Add item name",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "price",
					Description: "desired price",
					Type:        discordgo.ApplicationCommandOptionInteger,
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
			Name:        "edit_facebook_crawl",
			Description: "weather to crawl marketplace for this item or not",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "old_name",
					Description:  "name of item to be changed",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "crawl",
					Description: "bool",
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Required:    true,
				},
			},
		},
		{
			Name:        "add_additional_name",
			Description: "Add Additional Names for Tracking regex",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "name of item to be changed",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "additional_name",
					Description: "name of item to be changed",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "remove_alternative_name",
			Description: "Remove Additional Names for Tracking regex",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "name of item to be changed",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:         "index",
					Description:  "index of alternative name to remove",
					Type:         discordgo.ApplicationCommandOptionInteger,
					Required:     true,
					Autocomplete: true,
				},
			},
		},
		{
			Name:        "edit_alternative_name",
			Description: "Edit Additional Names for Tracking regex",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "name of item to be changed",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:         "index",
					Description:  "index of alternative name to edit",
					Type:         discordgo.ApplicationCommandOptionInteger,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "new_name",
					Description: "new name to replace the existing one",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "add_exclusion_query",
			Description: "Add Exclusion Query for Tracking regex",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "name of item",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:        "exclusion_query",
					Description: "exclusion pattern to add (regex string)",
					Type:        discordgo.ApplicationCommandOptionString,
					Required:    true,
				},
			},
		},
		{
			Name:        "remove_exclusion_query",
			Description: "Remove Exclusion Query for Tracking regex",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:         "name",
					Description:  "name of item",
					Type:         discordgo.ApplicationCommandOptionString,
					Required:     true,
					Autocomplete: true,
				},
				{
					Name:         "index",
					Description:  "index of exclusion query to remove",
					Type:         discordgo.ApplicationCommandOptionInteger,
					Required:     true,
					Autocomplete: true,
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
							Type:         discordgo.ApplicationCommandOptionInteger,
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
			Name:        "get_failure_report",
			Description: "Get failure report with incident charts",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "days",
					Description: "how many days of history to include",
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
			Name:        "channel_item_summary_one_week",
			Description: "Get Aggregate Data for the Used Listings of the Item",
		},
		{
			Name:        "channel_item_summary_custom_ln",
			Description: "Get Aggregate Data for the Used Listings of the Item",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "months",
					Description: "How Many Months Back for the Aggregations",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
				},
			},
		},
		{
			Name:        "restart",
			Description: "Saves Progress and Stops the Bot",
		},
		{
			Name:        "help",
			Description: "Show help and available commands",
		},
	}
)

var commandHandler = map[string]func(discord *discordgo.Session, i *discordgo.InteractionCreate){
	"setup": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		options := i.ApplicationCommandData().Options
		location := options[0].StringValue()
		locationCode := options[1].StringValue()
		maxDistance := int(options[2].IntValue())

		// add tracker to database
		err := database.UpdateChannelOrCreateChannelItemTableIfMissing(i.ChannelID,
			location,
			locationCode,
			maxDistance)
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
	"channel_info": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		info := database.GetChannelInfo(i.ChannelID)
		em := formatChannelInfo(info)
		err := discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{em},
			},
		})
		if err != nil {
			slog.Error("Error in Sending ChannelInfo", slog.Any("error", err))
		}
	},
	"channel_item_summary_one_week": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		customAcknowledge(discord, i)
		table := charts.ThisWeekAggregateTable(i.ChannelID)
		for _, string := range table {
			_, err := discord.ChannelMessageSend(i.ChannelID, string)
			if err != nil {
				slog.Error("Error in Sending ChannelInfo", slog.Any("error", err))
			}
		}
	},
	"channel_item_summary_custom_ln": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		customAcknowledge(discord, i)
		monthsBack := int(i.ApplicationCommandData().Options[0].IntValue())
		table := charts.CustomAggregateTable(i.ChannelID, monthsBack)
		for _, string := range table {
			_, err := discord.ChannelMessageSend(i.ChannelID, string)
			if err != nil {
				slog.Error("Error in Sending ChannelInfo", slog.Any("error", err))
			}
		}
	},
	"get_logs": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		customAcknowledge(discord, i)
		crawlType := i.ApplicationCommandData().Options[0].StringValue()
		CrawlErrorAlert("Logs", "User Requested",
			errors.New(crawlType), i.ChannelID)
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
			// 0 is item name, 1 is uri, 2 is htmlqueryselector, 3 is timer, 4 is type
			itemName := options[0].StringValue()
			uri := options[1].StringValue()
			htmlQuery := options[2].StringValue()
			timer := int(options[3].IntValue())
			itemType := options[4].StringValue()
			content := ""
			var em []*discordgo.MessageEmbed
			// add tracker to database
			addRes, err := database.AddItem(itemName, uri, htmlQuery, itemType, timer,
				database.ChannelMap[i.ChannelID],
			)
			if err != nil {
				content = fmt.Sprint(err)
				discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
					Content: "Error adding item" + content,
				})
				CrawlErrorAlert(itemName, uri, err, i.ChannelID)
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
			itemName := options[0].StringValue()
			newTimer := int(options[1].IntValue())
			err := database.EditTimer(itemName, newTimer, i.ChannelID)
			content := ""
			if err != nil {
				content = err.Error()
			} else {
				content = fmt.Sprintf("Price Update Notification Timer: %d Hours", newTimer)
			}
			discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: content,
			})
		}
	},
	"edit_facebook_crawl": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
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
			itemName := options[0].StringValue()
			enableCrawl := options[1].BoolValue()
			err := database.SetFacebookCrawl(itemName, enableCrawl, i.ChannelID)
			content := ""
			if err != nil {
				content = err.Error()
			} else {
				content = fmt.Sprintf("Facebook crawl value: %t", enableCrawl)
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
			itemName := options[0].StringValue()
			suppressNotifications := options[1].BoolValue()
			err := database.EditSuppress(itemName, suppressNotifications, i.ChannelID)
			content := ""
			if err != nil {
				content = err.Error()
			} else {
				content = fmt.Sprintf("Price Update Notification Status: %t", suppressNotifications)
			}
			discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: content,
			})
		}
	},
	"add_additional_name": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
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
			itemName := i.ApplicationCommandData().Options[0].StringValue()
			additionalName := i.ApplicationCommandData().Options[1].StringValue()
			err := database.AddAlternateTrackingName(itemName, additionalName, i.ChannelID)
			content := ""
			if err != nil {
				content = err.Error()
			} else {
				content = "Additional Tracking Names Added"
			}
			discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: content,
			})
		}
	},
	"add_exclusion_query": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(options[0].StringValue(), 0, i, discord)
		default:
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// add exclusion query to database
			itemName := i.ApplicationCommandData().Options[0].StringValue()
			exclusionQuery := i.ApplicationCommandData().Options[1].StringValue()
			err := database.AddTrackingExclusionQuery(itemName, exclusionQuery, i.ChannelID)
			content := ""
			if err != nil {
				content = err.Error()
			} else {
				content = "Exclusion Query Added"
			}
			discord.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
				Content: content,
			})
		}
	},
	"remove_exclusion_query": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			switch {
			case options[0].Focused:
				autoComplete(options[0].StringValue(), 0, i, discord)
			case options[1].Focused:
				autoComplete(options[0].StringValue(), 4, i, discord)
			}
		default:
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// remove exclusion query from database
			name := options[0].StringValue()
			index := int(options[1].IntValue())
			res, err := database.RemoveTrackingExclusionQuery(name, index, i.ChannelID)
			em := setEmbed(&res)
			if err != nil {
				content := err.Error()
				discord.ChannelMessageSendEmbed(i.ChannelID, &discordgo.MessageEmbed{
					Title: "Error",
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:  "Error",
							Value: content,
						},
					},
					Color: 10038562, // red
				})
			} else {
				for _, embed := range em {
					discord.ChannelMessageSendEmbed(i.ChannelID, embed)
				}
			}
		}
	},
	"remove_alternative_name": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			switch {
			case options[0].Focused:
				autoComplete(options[0].StringValue(), 0, i, discord)
			case options[1].Focused:
				autoComplete(options[0].StringValue(), 3, i, discord)
			}
		default:
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// remove alternative tracking name from database
			name := options[0].StringValue()
			index := int(options[1].IntValue())
			res, err := database.RemoveAlternateTrackingName(name, index, i.ChannelID)
			em := setEmbed(&res)
			if err != nil {
				content := err.Error()
				discord.ChannelMessageSendEmbed(i.ChannelID, &discordgo.MessageEmbed{
					Title: "Error",
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:  "Error",
							Value: content,
						},
					},
					Color: 10038562, // red
				})
			} else {
				for _, embed := range em {
					discord.ChannelMessageSendEmbed(i.ChannelID, embed)
				}
			}
		}
	},
	"edit_alternative_name": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			switch {
			case options[0].Focused:
				autoComplete(options[0].StringValue(), 0, i, discord)
			case options[1].Focused:
				autoComplete(options[0].StringValue(), 3, i, discord)
			}
		default:
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// edit alternative tracking name in database
			name := options[0].StringValue()
			index := int(options[1].IntValue())
			newName := options[2].StringValue()
			res, err := database.EditAlternateTrackingName(name, index, newName, i.ChannelID)
			em := setEmbed(&res)
			if err != nil {
				content := err.Error()
				discord.ChannelMessageSendEmbed(i.ChannelID, &discordgo.MessageEmbed{
					Title: "Error",
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:  "Error",
							Value: content,
						},
					},
					Color: 10038562, // red
				})
			} else {
				for _, embed := range em {
					discord.ChannelMessageSendEmbed(i.ChannelID, embed)
				}
			}
		}
	},
	"get": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		itemName := options[0].StringValue()
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(itemName, 0, i, discord)
		default:
			err := customAcknowledge(discord, i)
			if err != nil {
				slog.Error("ack error", slog.Any("error value", err))
			}
			getRes, err := database.GetItem(itemName, i.ChannelID, "PriceHistory", "ListingsHistory")
			if err != nil {
				SendErrorEmbed(discord, i.ChannelID, err.Error())
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
	"set_price": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		itemName := options[0].StringValue()
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(itemName, 0, i, discord)
		default:
			desiredPrice := int(options[1].IntValue())
			err := customAcknowledge(discord, i)
			if err != nil {
				slog.Error("ack error", slog.Any("error value", err))
			}
			err = database.SetDesiredPrice(itemName, i.ChannelID, desiredPrice)
			if err != nil {
				SendErrorEmbed(discord, i.ChannelID, err.Error())
			} else {
				SendSuccessEmbed(discord, i.ChannelID, fmt.Sprintf("Price for '%s' set to $%d", itemName, desiredPrice))
			}
		}
	}, "edit_name": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		oldName := options[0].StringValue()

		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(oldName, 0, i, discord)
			return
		case discordgo.InteractionApplicationCommand:
			newName := options[1].StringValue()
			getRes, err := database.EditName(oldName, newName, i.ChannelID)
			var embedArr []*discordgo.MessageEmbed
			var content string
			customAcknowledge(discord, i)
			if err != nil {
				content = "Error: " + err.Error()
				discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
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
		getRes := database.GetAllItems(i.ChannelID,
			[]string{"ListingsHistory", "PriceHistory"})
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
		itemName := options[0].StringValue()
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(itemName, 0, i, discord)
		default:
			// remove tracker to database
			deleteRes := database.RemoveItem(itemName, i.ChannelID)

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
			err := customAcknowledge(discord, i)
			content := ""
			// get option values
			name := options[0].Options[0].StringValue()

			// handle add and remove subcommands
			switch options[0].Name {
			case "add":
				uri := options[0].Options[1].StringValue()
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
					Discord.ChannelMessageSendEmbed(i.ChannelID, &discordgo.MessageEmbed{
						Title: "Error",
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:  "Error",
								Value: content,
							},
						},
						Color: 10038562, // red
					})
				} else {
					for _, embed := range em {
						discord.ChannelMessageSendEmbed(i.ChannelID, embed)
					}
				}

			case "remove":
				trackerIndex := options[0].Options[1].IntValue()
				res, err := database.RemoveTrackingInfo(name, int(trackerIndex), i.ChannelID)
				em := setEmbed(&res)
				if err != nil {
					content = err.Error()
					Discord.ChannelMessageSendEmbed(i.ChannelID, &discordgo.MessageEmbed{
						Title: "Error",
						Fields: []*discordgo.MessageEmbedField{
							{
								Name:  "Error",
								Value: content,
							},
						},
						Color: 10038562, // red
					})
				} else {
					for _, embed := range em {
						discord.ChannelMessageSendEmbed(i.ChannelID, embed)
					}
				}

			}

			if err != nil {
				slog.Error("Error in sending edit tracking response", slog.Any("Error", err))
			}
		}
	},
	"graph": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		itemName := options[0].StringValue()

		// handle autocomplete for name and normal request
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(itemName, 0, i, discord)
		default:
			// set up response to discord client
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// get command inputs from discord
			months := int(options[1].IntValue())
			err := charts.PriceHistoryChart([]string{itemName}, months, i.ChannelID)
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
		firstItemName := options[0].StringValue()
		secondItemName := options[1].StringValue()
		// handle autocomplete for name and normal request
		switch i.Type {

		case discordgo.InteractionApplicationCommandAutocomplete:
			switch {
			case options[0].Focused:
				autoComplete(firstItemName, 0, i, discord)
			case options[1].Focused:
				autoComplete(secondItemName, 0, i, discord)
			}

		default:
			// set up response to discord client
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// get command inputs from discord
			months := int(options[2].IntValue())
			err := charts.PriceHistoryChart([]string{
				firstItemName,
				secondItemName,
			}, months, i.ChannelID)
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
	"get_failure_report": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		days := int(options[0].IntValue())

		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Generating failure report for the last " + fmt.Sprint(days) + " days...",
			},
		})

		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -days)

		domainData, err := database.GetIncidentsByDomainOverTime(startDate, endDate)
		if err != nil {
			discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprint(err),
			})
			return
		}

		methodProxyData, err := database.GetIncidentsByDomainMethodProxy(startDate, endDate)
		if err != nil {
			discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: fmt.Sprint(err),
			})
			return
		}

		if len(domainData) == 0 && len(methodProxyData) == 0 {
			discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "No incident data found for the specified time period.",
			})
			return
		}

		if len(domainData) > 0 {
			err = charts.IncidentsByDomainChart(domainData)
			if err != nil {
				slog.Error("failed to generate domain chart", slog.Any("error", err))
			}
		}

		if len(methodProxyData) > 0 {
			err = charts.IncidentsByDomainMethodProxyChart(methodProxyData)
			if err != nil {
				slog.Error("failed to generate method proxy chart", slog.Any("error", err))
			}
		}

		var files []*discordgo.File

		if len(domainData) > 0 {
			reader, err := os.Open("incidents_by_domain.png")
			if err == nil {
				files = append(files, &discordgo.File{
					Name:        "incidents_by_domain.png",
					ContentType: "image/png",
					Reader:      reader,
				})
			}
		}

		if len(methodProxyData) > 0 {
			reader, err := os.Open("incidents_by_domain_method_proxy.png")
			if err == nil {
				files = append(files, &discordgo.File{
					Name:        "incidents_by_domain_method_proxy.png",
					ContentType: "image/png",
					Reader:      reader,
				})
			}
		}

		if len(files) > 0 {
			_, err = discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
				Content: "**Incidents by Domain Over Time (Last " + fmt.Sprint(days) + " days)**",
				Files:   files,
			})
			if err != nil {
				slog.Error("failed to send failure report", slog.Any("error", err))
			}
		}
	},
	"aggregate": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		itemName := options[0].StringValue()

		// handle autocomplete for name and normal request
		switch i.Type {
		case discordgo.InteractionApplicationCommandAutocomplete:
			autoComplete(itemName, 0, i, discord)
		default:
			// set up response to discord client
			discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			})
			// get command inputs from discord
			//
			monthsDuration := int(options[1].IntValue())
			endingMonthOffset := int(options[2].IntValue())
			endDate := time.Now().AddDate(0, -1*endingMonthOffset, 0)
			Aggregate, err := database.GenerateSecondHandPriceReport(
				itemName,
				endDate,
				monthsDuration*30, i.ChannelID)
			content := ""
			var fields []*discordgo.MessageEmbedField
			if err != nil {
				content = err.Error()
			} else {
				startDate := endDate.AddDate(0, 0, -30*monthsDuration)
				message := startDate.Format("2006-01-02") + " - " + endDate.Format("2006-01-02")
				fields = formatAggregateFields(&Aggregate, message)
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
	"help": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "**PriceTracker Commands** 🎉\n\n" +
					"**Setup & Configuration:**\n" +
					"• `/setup` - Configure channel location and marketplace settings\n" +
					"• `/channel_info` - View current channel settings\n\n" +
					"**Item Management:**\n" +
					"• `/add` - Add a new item to track\n" +
					"• `/remove` - Remove an item from tracking\n" +
					"• `/list` - List all tracked items\n" +
					"• `/get` - Get item prices and details\n\n" +
					"**Advanced Tracking:**\n" +
					"• `/add_additional_name` - Add alternate names for tracking\n" +
					"• `/remove_alternative_name` - Remove alternate names\n" +
					"• `/edit_alternative_name` - Edit alternate names\n" +
					"• `/add_exclusion_query` - Add exclusion patterns\n" +
					"• `/remove_exclusion_query` - Remove exclusion patterns\n\n" +
					"**Editing Trackers:**\n" +
					"• `/edit_name` - Rename an item\n" +
					"• `/edit_timer` - Change scrape interval\n" +
					"• `/edit_facebook_crawl` - Toggle Facebook crawling\n" +
					"• `/suppress` - Enable/disable notifications\n" +
					"• `/set_price` - Manually set desired price\n" +
					"• `/edit_tracking` - Add/remove tracking URLs\n\n" +
					"**Analytics & Visualization:**\n" +
					"• `/graph` - Price history chart\n" +
					"• `/graph-compare` - Compare two items\n" +
					"• `/aggregate` - Second-hand market statistics\n" +
					"• `/get_failure_report` - Crawler failure report\n" +
					"• `/channel_item_summary_one_week` - Weekly summary\n" +
					"• `/channel_item_summary_custom_ln` - Custom period summary\n\n" +
					"**Utilities:**\n" +
					"• `/get_logs` - Get debug logs and screenshots\n" +
					"• `/restart` - Restart the bot\n\n" +
					"For more information, visit: https://github.com/niknak1379/discord-price-tracker",
			},
		})
	},
}

// channelDeleteHandler deletes the channel from DB
// when user deltes a channel
// Parameters
//   - discord: session object
//   - i: channeldlete interaction
func channelDeleteHandler(discord *discordgo.Session, i *discordgo.ChannelDelete) {
	slog.Info("Channel being deleted with id: ", slog.String("ChannelID", i.Channel.ID))
	database.ChannelDeleteHandler(i.Channel.ID)
}

// guildCreateHandler handles when the bot joins a new guild.
// Sends a welcome message if this is the first time joining.
// Parameters:
//   - s: the session object
//   - g: server the bot is being added to
func guildCreateHandler(s *discordgo.Session, g *discordgo.GuildCreate) {
	if database.IsFirstTimeJoin(g.ID, g.SystemChannelID) {
		slog.Info("First time join for guild", slog.String("GuildID", g.ID))

		// Get system channel or find first accessible channel
		channelID := g.SystemChannelID
		if channelID == "" {
			for _, channel := range g.Channels {
				if channel.Type == discordgo.ChannelTypeGuildText {
					channelID = channel.ID
					break
				}
			}
		}

		if channelID != "" {
			sendWelcomeMessage(s, channelID)
		} else {
			slog.Error("could not find channel to send welcome message",
				slog.String("GuildID", g.ID))
		}
	}
}

// Run starts the Discord bot, registers commands, and handles interactions.
// It blocks until the context is cancelled, then gracefully shuts down.
//
// Parameters:
//   - ctx: the context for managing the bot lifecycle
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
	Discord.AddHandler(guildCreateHandler)

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
