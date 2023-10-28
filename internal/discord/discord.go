package discord

import (
	"fmt"

	"github.com/brandonlbarrow/jaggerbot/internal/twitchws"
	"github.com/bwmarrin/discordgo"
)

type Config struct {
	DiscordBotToken   string
	DiscordChannelIDs []string
	AdminChannelIDs   []string
	DiscordGuildID    string
	EventChannel      chan twitchws.Event
}

type Client struct {
	session         *discordgo.Session
	guildID         string
	channelIDs      []string
	adminChannelIDs []string
	eventChan       chan twitchws.Event
}

func NewClient(config *Config) (*Client, error) {
	session, err := discordgo.New("Bot " + config.DiscordBotToken)
	if err != nil {
		return nil, fmt.Errorf("error creating discord client: %w", err)
	}
	return &Client{
		session:    session,
		guildID:    config.DiscordGuildID,
		channelIDs: config.DiscordChannelIDs,
		eventChan:  config.EventChannel,
	}, nil
}

func (c *Client) Run() error {
	c.session.AddHandler(c.infoHandler)
	if err := c.session.Open(); err != nil {
		return fmt.Errorf("error opening or continuing websocket connection to discord: %w", err)
	}

	return nil
}

func (c *Client) infoHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}
	// for _, channelID := range c.channelIDs {
	// 	if strings.HasPrefix("!info", m.Content) && m.ChannelID == channelID {
	// 		c.session.ChannelMessageSend(channelID, "I'm Jagger!")
	// 	}
	// }
}

func (c *Client) SendMessage(content string) {
	for _, channelID := range c.channelIDs {
		c.session.ChannelMessageSend(channelID, content)
	}
}

func (c *Client) SendAdminMessage(err string) {
	for _, channelID := range c.adminChannelIDs {
		c.session.ChannelMessageSend(channelID, err)
	}
}
