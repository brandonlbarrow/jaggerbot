package discord

import (
	"fmt"
	"time"

	"github.com/brandonlbarrow/jaggerbot/internal/twitchws"
	"github.com/bwmarrin/discordgo"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
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
	start           time.Time
}

func NewClient(config *Config) (*Client, error) {
	session, err := discordgo.New("Bot " + config.DiscordBotToken)
	if err != nil {
		return nil, fmt.Errorf("error creating discord client: %w", err)
	}
	return &Client{
		session:         session,
		guildID:         config.DiscordGuildID,
		channelIDs:      config.DiscordChannelIDs,
		adminChannelIDs: config.AdminChannelIDs,
		eventChan:       config.EventChannel,
		start:           time.Now(),
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
	// if strings.HasPrefix("!info", m.Content) {
	// 	c.session.ChannelMessageSendEmbed(m.ChannelID, c.infoEmbed(""))
	// } else {
	// 	return
	// }
}

func (c *Client) SendMessage(content string) {
	for _, channelID := range c.channelIDs {
		c.session.ChannelMessageSend(channelID, content)
	}
}

func (c *Client) SendAdminMessage(content string) {
	for _, channelID := range c.adminChannelIDs {
		c.session.ChannelMessageSend(channelID, content)
	}
}

func (c *Client) infoEmbed(content string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:     "JaggerBot, a SensaiOpti Product",
		Timestamp: time.Now().Format(time.RFC3339),
		Color:     0x33ff33,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Uptime",
				Value:  time.Since(c.start).String(),
				Inline: true,
			}, {
				Name:   "GitHub",
				Value:  "https://github.com/brandonlbarrow/jaggerbot",
				Inline: true,
			}, {
				Name:   "CPU Util",
				Value:  getCPUPercent(),
				Inline: true,
			}, {
				Name:   "Memory Util",
				Value:  getMemoryUsage(),
				Inline: true,
			},
		},
	}
}

func getCPUPercent() string {
	val, err := cpu.Percent(0, false)
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	if len(val) == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f%%", val[0])
}

func getMemoryUsage() string {
	val, err := mem.VirtualMemory()
	if err != nil {
		return fmt.Sprintf("%v", err)
	}
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%.2f%%", val.UsedPercent)
}
