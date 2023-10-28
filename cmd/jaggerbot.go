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
	go runCallbackServer(eventChan, done)
	discordClient.SendAdminMessage("started jagger webserver...")
	_, err = twitchws.NewClient()
	if err != nil {
		discordClient.SendAdminMessage(fmt.Sprintf("jagger ran into an error. OOPSIE WOOPSIE! %s", err.Error()))
		log.Fatalf("error creating twitch client, cannot continue: %s", err)
	}
	for {
		var event twitchws.Event
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
			discordClient.SendMessage("get in here, Sensai's shitting it up! https://twitch.tv/sensaiopti")
		}
	}

}

func runDiscordClient(client *discord.Client, done chan error) error {
	if err := client.Run(); err != nil {
		done <- fmt.Errorf("error running discordgo session, %w", err)
	}
	return nil
}

func runCallbackServer(eventChan chan twitchws.Event, done chan error) error {
	server := http.NewServeMux()
	handler := webserver.Handler{EventChannel: eventChan}
	server.HandleFunc("/jagger/callback", handler.HandleTwitchCallback)
	log.Println("listening on :8080 for /jagger/callback")
	if err := http.ListenAndServe(":8080", server); err != nil {
		done <- fmt.Errorf("error running callback server: %w", err)
	}
	return nil
}
