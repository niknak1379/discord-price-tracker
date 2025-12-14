package bot

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var BotToken string

var commandList = []*discordgo.ApplicationCommand{
	{
		Name:        "add_tracker",
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
		Name:        "list_tracker_links",
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
		Name:        "remove_tracker",
		Description: "Remove Price Tracker",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "item_name",
				Description: "Item Name to be removed",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
		},
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
					{
						Name:        "html_tag",
						Description: "Add Scrapping HTML Tag",
						Type:        discordgo.ApplicationCommandOptionString,
						Required:    true,
					},
				},
			},
		},
	},
}
var commandHandler = map[string]func(discord *discordgo.Session, i *discordgo.InteractionCreate){
	"add_tracker": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				// three options for the three that were required by the command definition
				Content: fmt.Sprintf("%s , %s, %s", options[0].Name, options[1].Name, options[2].Value),
			},
		})
	},
	"remove_tracker": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Removing Tracker, will confirm later",
			},
		})
	},
	"remove_item": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Removing item from catalog",
			},
		})
	},
	"edit_tracker": func(discord *discordgo.Session, i *discordgo.InteractionCreate) {
		options := i.ApplicationCommandData().Options
		content := ""

		// As you can see, names of subcommands (nested, top-level)
		// and subcommand groups are provided through the arguments.
		switch options[0].Name {
		case "add_additional_tracking":
			content = "The top-level subcommand is executed. Now try to execute the nested one."
		case "remove_existing_tracking_option":
			content = "remove existing"
		}

		discord.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
			},
		})
	},
}

func checkNilErr(e error) {
	if e != nil {
		log.Fatal("Error message", e)
	}
}

func Run() {

	// create a session
	discord, err := discordgo.New("Bot " + BotToken)
	checkNilErr(err)

	discord.AddHandler(ready)
	// add a event handler
	discord.AddHandler(newMessage)

	// open session
	discord.Open()
	discord.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandler[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commandList))
	for index, command := range commandList {
		cmd, err := discord.ApplicationCommandCreate(discord.State.User.ID, "", command)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", command.Name, err)
		}
		registeredCommands[index] = cmd
	}

	defer discord.Close()
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
			err := discord.ApplicationCommandDelete(discord.State.User.ID, "", v.ID)
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
