package twitchws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/spddl/go-twitch-ws"
)

const (
	twitchEventSubURL                          = "wss://eventsub.wss.twitch.tv/ws"
	twitchUsername                             = "jaggerOpti"
	twitchEventSubscriptionsURL                = "https://api.twitch.tv/helix/eventsub/subscriptions"
	twitchGetUsersURL                          = "https://api.twitch.tv/helix/users"
	twitchAuthURL                              = "https://id.twitch.tv/oauth2/token"
	twitchGetChannelInfoURL                    = "https://api.twitch.tv/helix/channels"
	twitchEventSubscriptionStreamOnlineType    = "stream.online"
	twitchEventSubscriptionStreamOnlineVersion = "1"
)

type Client struct {
	ircClient *twitch.Client
}

func NewIRCClient(channelName string) (*Client, error) {
	bot, err := twitch.NewClient(&twitch.Client{
		Server:      twitchEventSubURL,
		User:        twitchUsername,
		Debug:       true,
		BotVerified: false,                                  // verified bots: Have higher chat limits than regular users.
		Channel:     []string{strings.ToLower(channelName)}, // only in Lowercase
	})
	if err != nil {
		return nil, fmt.Errorf("error creating Twitch EventSub client: %w", err)
	}
	return &Client{ircClient: bot}, nil
}

type WebsocketMessage struct {
	WebsocketMessageMetadata WebsocketMessageMetadata `json:"metadata"`
	WebsocketMessagePayload  WebsocketMessagePayload  `json:"payload"`
}

type WebsocketMessageMetadata struct {
	MessageID        string `json:"message_id"`
	MessageType      string `json:"message_type"`
	MessageTimestamp string `json:"message_timestamp"`
}

type WebsocketMessagePayload struct {
	Session WebsocketMessageSession `json:"session"`
}

type WebsocketMessageSession struct {
	ID                      string `json:"id"`
	Status                  string `json:"status"`
	ConnectedAt             string `json:"connected_at"`
	KeepaliveTimeoutSeconds int    `json:"keepalive_timeout_seconds"`
	ReconnectURL            string `json:"reconnect_url"`
}

type AuthRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // seconds
}

type GetSubscriptionsResponse struct {
	Total int            `json:"total"`
	Data  []Subscription `json:"data"`
}

type GetChannelInformationResponse struct {
	Data []ChannelInfo `json:"data"`
}

type ChannelInfo struct {
	BroadcasterID               string   `json:"broadcaster_id"`
	BroadcasterLogin            string   `json:"broadcaster_login"`
	BroadcasterName             string   `json:"broadcaster_name"`
	BroadcasterLanguage         string   `json:"broadcaster_language"`
	GameName                    string   `json:"game_name"`
	GameID                      string   `json:"game_id"`
	Title                       string   `json:"title"`
	Delay                       uint     `json:"delay"`
	Tags                        []string `json:"tags"`
	ContentClassificationLabels []string `json:"content_classification_labels"`
	IsBrandedContent            bool     `json:"is_branded_content"`
}

type Subscription struct {
	ID        string                `json:"id"`
	Type      string                `json:"type"`
	Version   string                `json:"version"`
	Cost      int                   `json:"cost"`
	Condition map[string]string     `json:"condition"`
	Status    string                `json:"status"`
	Transport SubscriptionTransport `json:"transport"`
}

type SubscriptionTransport struct {
	Method    string `json:"method"`     //webhook or websocket
	Callback  string `json:"callback"`   //valid only for webhooks
	Secret    string `json:"secret"`     //valid only for webhooks
	SessionID string `json:"session_id"` //valid only for websockets
}

type Event struct {
	UserID             string `json:"user_id"`
	Username           string `json:"user_name"`
	UserLogin          string `json:"user_login"`
	BroadcastUserID    string `json:"broadcast_user_id"`
	BroadcastUsername  string `json:"broadcast_user_name"`
	BroadcastUserLogin string `json:"broadcast_user_login"`
	FollowedAt         string `json:"followed_at"`
}

func (c *Client) RunIRCClient() {
	c.ircClient.Run()
}

func SetupTwitch() error {

	authResp, err := authenticateToTwitch()
	if err != nil {
		return fmt.Errorf("error getting app token: %w", err)
	}
	userId := os.Getenv("TWITCH_SENSAI_USER_ID")
	_, err = getEventSubscriptions(authResp, true)
	if err != nil {
		return fmt.Errorf("error getting twitch subscriptions: %w", err)
	}
	if err := subscribeToSensai(authResp, os.Getenv("TWITCH_EVENTSUB_SECRET"), userId); err != nil {
		return fmt.Errorf("error subscribing to channel broadcast online: %w", err)
	}
	return nil
}

func GetEventSubscriptions() (*GetSubscriptionsResponse, error) {
	authResp, err := authenticateToTwitch()
	if err != nil {
		return nil, fmt.Errorf("error getting app token: %w", err)
	}
	return getEventSubscriptions(authResp, false)
}

func GetChannelInformation() (*GetChannelInformationResponse, error) {
	userId := os.Getenv("TWITCH_SENSAI_USER_ID")
	var getChannelInfoResp GetChannelInformationResponse
	token, err := authenticateToTwitch()
	if err != nil {
		return nil, fmt.Errorf("error authenticating to Twitch to get channel information: %w", err)
	}
	httpClient := http.DefaultClient
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s?broadcaster_id=%s", twitchGetChannelInfoURL, userId), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for getting channel information: %w", err)
	}
	req.Header.Add("Client-Id", os.Getenv("TWITCH_CLIENT_ID"))
	req.Header.Add("Authorization", fmt.Sprint("Bearer ", token))
	req.Header.Add("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending http request to get channel information: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code from response was not OK: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&getChannelInfoResp); err != nil {
		return nil, fmt.Errorf("could not decode response body from get channel info response: %w", err)
	}
	return &getChannelInfoResp, nil
}

func authenticateToTwitch() (string, error) {
	httpClient := http.DefaultClient
	authReq := AuthRequest{
		ClientID:     os.Getenv("TWITCH_CLIENT_ID"),
		ClientSecret: os.Getenv("TWITCH_BOT_TOKEN"),
		GrantType:    "client_credentials",
	}
	marshaledReq, err := json.Marshal(&authReq)
	if err != nil {
		return "", err
	}
	r := io.NopCloser(bytes.NewBuffer(marshaledReq))
	resp, err := httpClient.Post(twitchAuthURL, "application/json", r)
	if err != nil {
		return "", err
	}
	var authResp AuthResponse
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(bodyBytes, &authResp)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(authResp.AccessToken), nil
}

func subscribeToSensai(token, secret, channelID string) error {
	httpClient := http.DefaultClient
	subscriptionReq := Subscription{
		Type:    twitchEventSubscriptionStreamOnlineType,
		Version: twitchEventSubscriptionStreamOnlineVersion,
		Condition: map[string]string{
			"broadcaster_user_id": channelID,
		},
		Transport: SubscriptionTransport{
			Method:   "webhook",
			Secret:   secret,
			Callback: "https://gonkbot.brandonbarrow.com/jagger/callback",
		},
	}
	marshaledReqBody, err := json.Marshal(&subscriptionReq)
	if err != nil {
		return err
	}
	r := io.NopCloser(bytes.NewBuffer(marshaledReqBody))
	req, err := http.NewRequest(http.MethodPost, twitchEventSubscriptionsURL, r)
	if err != nil {
		return err
	}
	req.Header.Add("Client-Id", os.Getenv("TWITCH_CLIENT_ID"))
	req.Header.Add("Authorization", fmt.Sprint("Bearer ", token))
	req.Header.Add("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if strings.Contains(string(bodyBytes), "subscription already exists") {
		log.Println("subscription already exists, doing nothing")
	} else {
		log.Println(string(bodyBytes))
	}

	return nil
}

func getEventSubscriptions(token string, flush bool) (*GetSubscriptionsResponse, error) {
	httpClient := http.DefaultClient
	req, err := http.NewRequest(http.MethodGet, twitchEventSubscriptionsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("getEventSubscriptions: error forming http request: %w", err)
	}
	req.Header.Add("Client-Id", os.Getenv("TWITCH_CLIENT_ID"))
	req.Header.Add("Authorization", fmt.Sprint("Bearer ", token))
	req.Header.Add("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getEventSubscriptions: error making http request to %s: %w", twitchEventSubscriptionsURL, err)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("getEventSubscriptions: error reading response body: %w", err)
	}
	var getSubResp GetSubscriptionsResponse
	if err := json.Unmarshal(bodyBytes, &getSubResp); err != nil {
		return nil, fmt.Errorf("getEventSubscriptions: error unmarshaling to GetSubscriptionsResponse: %v: %w", string(bodyBytes), err)
	}
	for _, i := range getSubResp.Data {
		log.Println("status", i.Status)
	}
	if flush {
		for _, i := range getSubResp.Data {
			deleteEventSubscriptions(i.ID, token)
		}
	}

	return &getSubResp, nil
}

func deleteEventSubscriptions(subscriptionID, token string) error {
	httpClient := http.DefaultClient
	req, err := http.NewRequest(http.MethodDelete, twitchEventSubscriptionsURL, nil)
	if err != nil {
		return err
	}
	values := req.URL.Query()
	values.Set("id", subscriptionID)
	req.URL.RawQuery = values.Encode()
	req.Header.Add("Client-Id", os.Getenv("TWITCH_CLIENT_ID"))
	req.Header.Add("Authorization", fmt.Sprint("Bearer ", token))
	req.Header.Add("Content-Type", "application/json")
	_, err = httpClient.Do(req)
	if err != nil {
		return err
	}
	return nil
}
