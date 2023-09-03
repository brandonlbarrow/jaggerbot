package webserver

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/brandonlbarrow/jaggerbot/internal/twitchws"
)

const (
	TwitchEventsubMessageIDHeader        = "Twitch-Eventsub-Message-Id"
	TwitchEventsubMessageTimestampHeader = "Twitch-Eventsub-Message-Timestamp"
	TwitchEventsubMessageSignatureHeader = "Twitch-Eventsub-Message-Signature"
	TwitchEventsubMessageTypeHeader      = "Twitch-Eventsub-Message-Type"
)

type Handler struct {
	EventChannel chan twitchws.Event
}

func (h *Handler) HandleTwitchCallback(w http.ResponseWriter, r *http.Request) {

	log.Printf("got request")
	if err := verifyMessageSignature(w, r); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if r.Header.Get(TwitchEventsubMessageTypeHeader) == "webhook_callback_verification" {
		log.Printf("got webhook_callback_verification request")
		handleChallengeVerification(w, r)
		return
	}
	if r.Header.Get(TwitchEventsubMessageTypeHeader) == "notification" {
		log.Printf("got notification request")
		h.handleSubscriptionEventNotification(w, r)
		return
	}
}

func verifyMessageSignature(w http.ResponseWriter, r *http.Request) error {
	secret := os.Getenv("TWITCH_EVENTSUB_SECRET")
	incomingReqMessageID := r.Header.Get(TwitchEventsubMessageIDHeader)
	incomingReqMessageTimestamp := r.Header.Get(TwitchEventsubMessageTimestampHeader)
	incomingReqRawBody, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(incomingReqRawBody))

	if err != nil {
		return fmt.Errorf("error reading raw request body: %w", err)
	}
	hmacMessage := fmt.Sprintf("%s%s%s", incomingReqMessageID, incomingReqMessageTimestamp, incomingReqRawBody)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(hmacMessage))
	incomingSignature := fmt.Sprint("sha256=", hex.EncodeToString(h.Sum(nil)))
	providedSignature := r.Header.Get(TwitchEventsubMessageSignatureHeader)
	subtle.ConstantTimeCompare([]byte(incomingSignature), []byte(providedSignature))
	if incomingSignature != providedSignature {
		log.Println("signatures do not match")
		log.Println("incoming ", incomingSignature)
		log.Println("provided ", providedSignature)
		return fmt.Errorf("error signatures do not match")
	}
	return nil
}

func handleChallengeVerification(w http.ResponseWriter, r *http.Request) {
	log.Print("got challenge request")
	reqBody, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(reqBody))

	if err != nil {
		log.Printf("cannot read request body: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var challengeRequest challengeRequestBody
	if err := json.Unmarshal(reqBody, &challengeRequest); err != nil {
		log.Printf("cannot unmarshal request body: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", fmt.Sprint(len(challengeRequest.Challenge)))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(challengeRequest.Challenge))
}

type challengeRequestBody struct {
	Challenge    string                `json:"challenge"`
	Subscription twitchws.Subscription `json:"subscription"`
	CreatedAt    string                `json:"created_at"`
}

func (h *Handler) handleSubscriptionEventNotification(w http.ResponseWriter, r *http.Request) {
	log.Print("got subscription event notification request")
	reqBody, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("cannot read request body: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var event subscriptionEventNotificationRequest
	if err := json.Unmarshal(reqBody, &event); err != nil {
		log.Printf("cannot unmarshal request body: %s", err)
		log.Printf("%v", string(reqBody))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("event: %v\n", event)
	h.EventChannel <- event.Event
	w.WriteHeader(http.StatusOK)
}

type subscriptionEventNotificationRequest struct {
	Subscription twitchws.Subscription `json:"subscription"`
	Event        twitchws.Event        `json:"event"`
}
