package discord

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	charts "priceTracker/Charts"
	database "priceTracker/Database"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var BotToken string
var Discord *discordgo.Session
var commandList = []*discordgo.ApplicationCommand{
	{
		Name:        "add_item",
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
		Name:        "get_item",
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
		Name:        "get_all_items",
		Description: "get all items",
	},
	{
		Name:        "remove_item",
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
		Name:        "edit_tracker",
		Description: "Edit a currently Existing Tracker",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "add_additional_tracking",
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
				Name:        "remove_existing_tracking_option",
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
		Name:        "graph_price",
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
var commandHandler = map[string]func(discord *discordgo.Session, i *discordgo.InteractionCreate){
	"add_item": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		// 0 is item name, 1 is uri, 2 is htmlqueryselector

		// add tracker to database
		addRes := database.AddItem(options[0].StringValue(), options[1].StringValue(), options[2].StringValue())

		// set up response to discord client
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				// three options for the three that were required by the command definition
				Content: fmt.Sprint(addRes),
			},
		})
	},
	
	"get_item": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		// 0 is item name, 1 is uri, 2 is htmlqueryselector
		content := ""

		// add tracker to database
		getRes, err := database.GetItem(options[0].StringValue())
		var embedArr []*discordgo.MessageEmbed
		if (err != nil){
			content = err.Error()
		}else{
			var fields []*discordgo.MessageEmbedField
			for _,tracker := range getRes.TrackingList{
				field := discordgo.MessageEmbedField{
					Name: tracker.URI,
					Value: tracker.HtmlQuery,
					Inline: true,
				}
				fields = append(fields, &field)
			}
			em := discordgo.MessageEmbed{
				Title: getRes.Name,
				Fields: fields,
				Type: discordgo.EmbedTypeRich,
			}
			embedArr = append(embedArr, &em)
		}
		

		// set up response to discord client
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Embeds: embedArr,
			},
		})
	},
	"get_all_items": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		// add tracker to database
		getRes := database.GetAllItems()
		//returnstr, _ := json.Marshal(getRes)
		var embedArr []*discordgo.MessageEmbed
		for _, Item := range getRes{
			var fields []*discordgo.MessageEmbedField
			for _,tracker := range Item.TrackingList{
				field := discordgo.MessageEmbedField{
					Name: tracker.URI,
					Value: tracker.HtmlQuery,
					Inline: true,
				}
				fields = append(fields, &field)
			}
			em := discordgo.MessageEmbed{
				Title: Item.Name,
				Fields: fields,
				Type: discordgo.EmbedTypeRich,
			}
			embedArr = append(embedArr, &em)
		}
		
		// set up response to discord client
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Embeds: embedArr,
			},
		})
	},
	"remove_item": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
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
	"edit_tracker": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		content := ""

		// get option values
		name := options[0].Options[0].StringValue()
		uri := options[0].Options[1].StringValue()
		
		// As you can see, names of subcommands (nested, top-level)
		// and subcommand groups are provided through the arguments.
		switch options[0].Name {
		case "add_additional_tracking":
			htmlQuery := options[0].Options[2].StringValue()
			res, err := database.AddTrackingInfo(name, uri, htmlQuery)
			if (err != nil){
				content = err.Error()
			}else{
				content = fmt.Sprint(res)
			}
			
		case "remove_existing_tracking_option":
			res, err := database.RemoveTrackingInfo(name, uri)
			if (err != nil){
				content = err.Error()
			}else{
				content = fmt.Sprint(res)
			}
		}

		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
			},
		})
	},
	"graph_price": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		
		// set up response to discord client
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		})
		// get command inputs from discord
		options := i.ApplicationCommandData().Options
		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		logger.Info("options", slog.Any("optoin", options))
		err := charts.PriceHistoryChart(options[0].StringValue(), int(options[1].IntValue()))
		if (err != nil) {
			log.Print(err)
		}
		reader, err := os.Open("my-chart.png")
		if err != nil{
			log.Fatal(err)
		}
		File := discordgo.File{
			Name: "chart.png",
			ContentType: "Image",
			Reader: reader,
		}
		_, err = discord.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
			Files: []*discordgo.File{&File},
		})
		if err != nil {
			fmt.Printf("Error sending follow-up message: %v\n", err)
		}
	},
}

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Error message", e)
	}
}

func Run() {

	// create a session
	var err error
	Discord, err = discordgo.New("Bot " + BotToken)
	checkNilErr(err)

	Discord.AddHandler(ready)
	// add a event handler
	Discord.AddHandler(newMessage)

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

	defer Discord.Close()
	// this makes a chanel, channels are how routeines talk to each other
	// Notify, intercepts the signal and sends it to chanel c -> then you can
	// implement what to do with it, here it just processes the signal and lets
	// the function execution end

	// this runs after the interupt signal

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop
	fmt.Println("recieved signal, shutting down")
	if true {
		log.Println("Removing commands...")
		// // We need to fetch the commands, since deleting requires the command ID.
		// // We are doing this from the returned commands on line 375, because using
		// // this will delete all the commands, which might not be desirable, so we
		// // are deleting only the commands that we added.
		// registeredCommands, err := s.ApplicationCommands(s.State.User.ID, *GuildID)
		// if err != nil {
		// 	log.Fatalf("Could not fetch registered commands: %v", err)
		// }

		for _, v := range registeredCommands {
			err := Discord.ApplicationCommandDelete(Discord.State.User.ID, "", v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
		}
	}

	log.Println("Gracefully shutting down.")

}

func newMessage(discord *discordgo.Session, message *discordgo.MessageCreate) {

	/* prevent bot responding to its own message
	this is achived by looking into the message author id
	if message.author.id is same as bot.author.id then just return
	*/
	if message.Author.ID == discord.State.User.ID {
		return
	}
	fmt.Println("logging message", message.Content)
	fmt.Println("ID", message.ChannelID)

	// respond to user message if it contains `!help` or `!bye`
	switch {
	case strings.Contains(message.Content, "!help"):
		discord.ChannelMessageSend(message.ChannelID, "Hello World😃")
	case strings.Contains(message.Content, "!bye"):
		discord.ChannelMessageSend(message.ChannelID, "Good Bye👋")
		// add more cases if required
	}

}

func ready(discord *discordgo.Session, ready *discordgo.Ready) {
	fmt.Println("Logged in")
	discord.UpdateGameStatus(1, "stonks")
}

func LowestPriceAlert(discord *discordgo.Session, itemName string, newPrice int,oldPrice int, URL string){
	content := fmt.Sprintf("New Price Alert!!!!\nItem %s has hit its lowest price of %d " +
						   "from previous lowest of %d with the following url \n%s",
	itemName, newPrice, oldPrice, URL)
	discord.ChannelMessageSend("803818389755265075", content)
}
func CrawlErrorAlert(discord *discordgo.Session, itemName string, URL string, err error){
	content := fmt.Sprintf("Crawler could not find price for %s in url %s, with error %s investigate logs for further information",
	itemName, URL, err.Error())
	log.Printf("Crawler could not find price for %s in url %s, with error %s investigate logs for further information",
	itemName, URL, err.Error())
	discord.ChannelMessageSend(os.Getenv("CHANNEL_ID"), content)
}
func SendGraphPng(discord *discordgo.Session){
	//content := fmt.Sprintf("Chart for Product named %s for the last %d months", productName, months)
	reader, err := os.Open("my-chart.png")
	if err != nil{
		log.Fatal(err)
	}
	discord.ChannelFileSend(os.Getenv("CHANNEL_ID"), "my-chart.png", reader)
	
	
}
