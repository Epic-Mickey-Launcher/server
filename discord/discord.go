package discord

import (
	"emlserver/config"
	"emlserver/database"
	"emlserver/mail"
	"emlserver/structs"
	"emlserver/ticket"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	Client    *discordgo.Session
	channelID string
	roleID    string
	guildID   string
)

func BeginClient() error {
	channelID = config.LoadedConfig["DISCORD_ID"]
	roleID = config.LoadedConfig["DISCORD_ROLE"]
	guildID = config.LoadedConfig["DISCORD_GUILDID"]

	ticket.DiscordReportTicket = func(ticket structs.Ticket) {
		err := ReportTicket(ticket)
		if err != nil {
			return
		}
	}

	client, err := discordgo.New("Bot " + config.LoadedConfig["DISCORD_TOKEN"])
	if err != nil {
		return err
	}
	err = client.Open()
	if err != nil {
		return err
	}
	for _, v := range commands {
		_, err := client.ApplicationCommandCreate(client.State.User.ID, guildID, v)
		if err != nil {
			return err
		}
	}

	client.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})

	Client = client
	go SetStatusRoutine()
	return nil
}

func SetStatusRoutine() {
	for {
		modCount, err := database.GetModCount()
		if err != nil {
			continue
		}
		userCount, err := database.GetUserCount()
		if err != nil {
			continue
		}
		err = Client.UpdateCustomStatus(fmt.Sprint(userCount, " Users / ", modCount, " Mods"))
		if err != nil {
			continue
		}

		time.Sleep(10 * time.Second)
	}
}

func PrintTicket(ticket structs.Ticket) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{},
		Color:  0xa434eb,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Ticket ID",
				Value:  ticket.ID,
				Inline: true,
			},
			{
				Name:   "Target ID",
				Value:  ticket.TargetID,
				Inline: true,
			},
			{
				Name:   "Author",
				Value:  ticket.Author,
				Inline: true,
			},
			{
				Name:   "Meta",
				Value:  ticket.Meta,
				Inline: true,
			},
			{
				Name:   "Action",
				Value:  ticket.Action,
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Title:     ticket.Title,
	}
	return embed
}

func ReportTicket(ticket structs.Ticket) error {
	embed := PrintTicket(ticket)

	_, err := Client.ChannelMessageSendEmbed(channelID, embed)
	if err != nil {
		println("failed to send embed on ticket")
		return err
	}
	_, err = Client.ChannelMessageSend(channelID, "<@&"+roleID+">")
	if err != nil {
		println("failed to send ping on ticket")
		return err
	}
	return nil
}

func onMessage() {
}

func CloseTicket(ticketID string, result int, resultMessage string) error {
	_, err := database.Database.Exec("UPDATE tickets SET result=$1, resultmsg=$2 WHERE ticketid=$3", result, resultMessage, ticketID)
	return err
}

var (
	integerOptionMinValue          = 1.0
	dmPermission                   = true
	defaultMemberPermissions int64 = discordgo.PermissionAll

	commands = []*discordgo.ApplicationCommand{
		{
			Name:        "download",
			Description: "Get a download link of a mod by its ID",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "id",
					Description: "ID of the mod you want to download.",
					Required:    true,
				},
			},
		},
		{
			Name:        "testemail",
			Description: "Send a test email to a specified address to test server email connection.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "email-address",
					Description: "Address of the email you want to send the test mail to.",
					Required:    true,
				},
			},
		},
		{
			Name:        "ban",
			Description: "Ban a user by their ID",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "id",
					Description: "ID of the user you want to ban.",
					Required:    true,
				},
			},
		},
		{
			Name:        "get",
			Description: "Gets data by its ID",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "id",
					Description: "ID of the element you want to get info from.",
					Required:    true,
				},
			},
		},
		{
			Name:        "approve",
			Description: "Approves a ticket by its ID",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "ticket-id",
					Description: "ID of the ticket you want to approve.",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "additional-comment",
					Description: "Optional comment for the ticket author.",
					Required:    false,
				},
			},
		},
		{
			Name:        "deny",
			Description: "Denies a ticket by its ID",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "ticket-id",
					Description: "ID of the ticket you want to deny.",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "reason",
					Description: "Reason for denial.",
					Required:    true,
				},
			},
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"download": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			id := options[0].Value.(string)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: config.LoadedConfig["URL"] + "mod/download?id=" + id,
				},
			})
		},
		"testemail": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			email := options[0].Value.(string)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Test Email Sent",
				},
			})
			mail.TestEmail(email)
		},
		"ban": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			// options := i.ApplicationCommandData().Options
			// id := options[0].Value.(string)
		},
		"get": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options
			id := options[0].Value.(string)

			user, err := database.GetUser(id)
			if err == nil {
				embed := &discordgo.MessageEmbed{
					Author: &discordgo.MessageEmbedAuthor{},
					Color:  0xa434eb,
					Image: &discordgo.MessageEmbedImage{
						URL: fmt.Sprintf("%simg/userpfp?id=%s", config.LoadedConfig["URL"], user.ID),
					},
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "Bio",
							Value:  user.Bio,
							Inline: true,
						},
						{
							Name:   "ID (ms since epoch)",
							Value:  user.ID,
							Inline: true,
						},
					},
					Timestamp: time.Now().Format(time.RFC3339), // Discord wants ISO8601; RFC3339 is an extension of ISO8601 and should be completely compatible.
					Title:     user.Username,
				}
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Embeds: []*discordgo.MessageEmbed{
							embed,
						},
					},
				})
				return
			}
			mod, err := database.GetMod(id)

			if err == nil {
				embed := &discordgo.MessageEmbed{
					Author: &discordgo.MessageEmbedAuthor{},
					Color:  0xa434eb,
					Image: &discordgo.MessageEmbedImage{
						URL: fmt.Sprintf("%simg/modicon?id=%s", config.LoadedConfig["URL"], mod.ID),
					},
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "Description",
							Value:  mod.Description,
							Inline: true,
						},
						{
							Name:   "Downloads",
							Value:  fmt.Sprint(mod.Downloads),
							Inline: true,
						},
						{
							Name:   "Likes",
							Value:  fmt.Sprint(mod.CachedLikes),
							Inline: true,
						},
						{
							Name:   "YouTube Video",
							Value:  mod.Video,
							Inline: true,
						},
						{
							Name:   "Author",
							Value:  mod.Author,
							Inline: true,
						},
						{
							Name:   "Game",
							Value:  mod.Game,
							Inline: true,
						},

						{
							Name:   "Platform",
							Value:  mod.Platform,
							Inline: true,
						},
						{
							Name:   "Published",
							Value:  strconv.FormatBool(mod.Published),
							Inline: true,
						},
						{
							Name:   "Verified",
							Value:  strconv.FormatBool(mod.Verified),
							Inline: true,
						},
						{
							Name:   "Version",
							Value:  fmt.Sprint(mod.Version),
							Inline: true,
						},
						{
							Name:   "Dependencies",
							Value:  strings.Join(mod.Dependencies, ", "),
							Inline: true,
						},
						{
							Name:   "ID (ms since epoch)",
							Value:  mod.ID,
							Inline: true,
						},
					},
					Timestamp: time.Now().Format(time.RFC3339), // Discord wants ISO8601; RFC3339 is an extension of ISO8601 and should be completely compatible.
					Title:     mod.Name,
				}
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Embeds: []*discordgo.MessageEmbed{
							embed,
						},
					},
				})
				return
			}

			ticket, err := ticket.GetTicketFromTicketID(id)

			if err == nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Embeds: []*discordgo.MessageEmbed{
							PrintTicket(ticket),
						},
					},
				})
				return
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "ain't nothin with that ID in the database!",
				},
			})
		},

		"approve": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			id := options[0].Value.(string)
			optionsLength := len(options)
			var msg string
			if optionsLength > 1 {
				msg = options[1].Value.(string)
			}

			CloseTicket(id, 1, msg)
			err := ticket.OnTicketReview(id)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Error: " + err.Error(),
					},
				})
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Approved " + id + "!",
				},
			})
		},

		"deny": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			options := i.ApplicationCommandData().Options

			id := options[0].Value.(string)
			msg := options[1].Value.(string)

			ticket.CloseTicket(id, -1, msg)
			err := ticket.OnTicketReview(id)
			if err != nil {
				s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Error: " + err.Error(),
					},
				})
			}

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Denied " + id + "!",
				},
			})
		},
	}
)
