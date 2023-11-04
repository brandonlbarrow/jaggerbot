package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/brandonlbarrow/jaggerbot/internal/discord"
	"github.com/brandonlbarrow/jaggerbot/internal/twitchws"
	"github.com/brandonlbarrow/jaggerbot/internal/webserver"
	"github.com/joho/godotenv"
)

func main() {

	godotenv.Load()

	eventChan := make(chan twitchws.Event)
	errorEventChan := make(chan error)

	discordConfig := discord.Config{
		DiscordGuildID:    os.Getenv("DISCORD_GUILD_ID"),
		DiscordBotToken:   os.Getenv("DISCORD_BOT_TOKEN"),
		DiscordChannelIDs: strings.Split(os.Getenv("DISCORD_CHANNEL_IDS"), ","),
		AdminChannelIDs:   strings.Split(os.Getenv("DISCORD_ADMIN_CHANNEL_IDs"), ","),
		EventChannel:      eventChan,
	}

	discordClient, err := discord.NewClient(&discordConfig)
	if err != nil {
		log.Fatalf("error creating discord client, cannot continue: %s", err)
	}
	done := make(chan error)
	go runDiscordClient(discordClient, done)
	discordClient.SendAdminMessage("started jagger discord client...")
	go runCallbackServer(eventChan, errorEventChan, done)
	discordClient.SendAdminMessage("started jagger webserver...")
	if err = twitchws.SetupTwitch(); err != nil {
		discordClient.SendAdminMessage(fmt.Sprintf("jagger ran into an error. OOPSIE WOOPSIE! %s", err.Error()))
		log.Fatalf("error creating twitch client, cannot continue: %s", err)
	}
	if resp, err := twitchws.GetChannelInformation(); err != nil {
		log.Printf("could not get channel info: %s", err)
	} else {
		log.Println(resp)
	}
	for {
		var event twitchws.Event
		var errEvent error
		discordClient.SendAdminMessage("jagger is listening for Twitch events...")
		select {
		case runErr := <-done:
			if runErr != nil {
				discordClient.SendAdminMessage(fmt.Sprintf("jagger ran into an error. OOPSIE WOOPSIE! %s", runErr.Error()))
				log.Fatal(runErr)
			}
		case event = <-eventChan:
			log.Printf("received event: %v\n", event)
			discordClient.SendAdminMessage(fmt.Sprintf("jagger received an event from Twitch: \n%v", event))
			resp, err := twitchws.GetChannelInformation()
			if err != nil {
				discordClient.SendAdminMessage(fmt.Sprintf("jagger could not get channel information for stream announcement. Sending a normal message. Error: \n%s", err.Error()))
				discordClient.SendMessage("get in here, Sensai's shitting it up! https://twitch.tv/sensaiopti")
			} else if len(resp.Data) != 1 {
				discordClient.SendAdminMessage(fmt.Sprintf("jagger got game information but the response was not expected: %v", resp.Data))
			} else {
				var gameName, streamTitle string
				for _, d := range resp.Data {
					gameName = d.GameName
					streamTitle = d.Title
				}
				discordClient.SendMessageEmbed("get in here, Sensai's shitting it up!", gameName, streamTitle)
			}

		case errEvent = <-errorEventChan:
			log.Printf("webserver encountered error: %s", errEvent)
			discordClient.SendAdminMessage(fmt.Sprintf("jagger webserver had error handling Twitch event: %s", errEvent.Error()))
		}
	}

}

func runDiscordClient(client *discord.Client, done chan error) error {
	if err := client.Run(); err != nil {
		done <- fmt.Errorf("error running discordgo session, %w", err)
	}
	return nil
}

func runCallbackServer(eventChan chan twitchws.Event, errorEventChan, done chan error) error {
	server := http.NewServeMux()
	handler := webserver.Handler{EventChannel: eventChan, ErrorEventChannel: errorEventChan}
	server.HandleFunc("/jagger/callback", handler.HandleTwitchCallback)
	log.Println("listening on :8080 for /jagger/callback")
	if err := http.ListenAndServe(":8080", server); err != nil {
		done <- fmt.Errorf("error running callback server: %w", err)
	}
	return nil
}
