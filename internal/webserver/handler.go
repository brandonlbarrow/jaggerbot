package webserver

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	//"crypto/subtle"
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
	EventChannel      chan twitchws.Event
	ErrorEventChannel chan error
}

func (h *Handler) HandleTwitchCallback(w http.ResponseWriter, r *http.Request) {

	log.Printf("got request")
	if err := h.verifyMessageSignature(w, r); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if r.Header.Get(TwitchEventsubMessageTypeHeader) == "webhook_callback_verification" {
		log.Printf("got webhook_callback_verification request")
		h.handleChallengeVerification(w, r)
		return
	}
	if r.Header.Get(TwitchEventsubMessageTypeHeader) == "notification" {
		log.Printf("got notification request")
		h.handleSubscriptionEventNotification(w, r)
		return
	}
}

func (h *Handler) verifyMessageSignature(w http.ResponseWriter, r *http.Request) error {
	secret := os.Getenv("TWITCH_EVENTSUB_SECRET")
	incomingReqMessageID := r.Header.Get(TwitchEventsubMessageIDHeader)
	incomingReqMessageTimestamp := r.Header.Get(TwitchEventsubMessageTimestampHeader)
	incomingReqRawBody, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(incomingReqRawBody))

	if err != nil {
		respErr := fmt.Errorf("verifyMessageSignature: error reading raw request body: %w\nDetails: RequestMessageID: %s\nRequestTimestamp:%s", err, incomingReqMessageID, incomingReqMessageTimestamp)
		log.Println(respErr)
		h.ErrorEventChannel <- respErr
		return respErr
	}
	hmacMessage := fmt.Sprintf("%s%s%s", incomingReqMessageID, incomingReqMessageTimestamp, incomingReqRawBody)
	hm := hmac.New(sha256.New, []byte(secret))
	hm.Write([]byte(hmacMessage))
	incomingSignature := fmt.Sprint("sha256=", hex.EncodeToString(hm.Sum(nil)))
	providedSignature := r.Header.Get(TwitchEventsubMessageSignatureHeader)
	//subtle.ConstantTimeCompare([]byte(incomingSignature), []byte(providedSignature))
	if incomingSignature != providedSignature {
		respErr := fmt.Errorf("verifyMessageSignature: error signatures do not match:\nDetails: RequestBody: %v\nRequestSignature: %s\nProvidedSignature: %s\nRequestMessageID: %s\nRequestTimestamp: %v",
			incomingReqRawBody, incomingSignature, providedSignature, incomingReqMessageID, incomingReqMessageTimestamp)
		log.Println(respErr)
		h.ErrorEventChannel <- respErr
		return respErr
	}
	return nil
}

func (h *Handler) handleChallengeVerification(w http.ResponseWriter, r *http.Request) {
	log.Print("got challenge request")
	reqBody, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewBuffer(reqBody))

	if err != nil {
		respErr := fmt.Errorf("handleChallengeVerification: cannot read request body: %w", err)
		log.Println(respErr)
		h.ErrorEventChannel <- respErr
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var challengeRequest challengeRequestBody
	if err := json.Unmarshal(reqBody, &challengeRequest); err != nil {
		respErr := fmt.Errorf("handleChallengeVerification: cannot unmarshal request body: %w\nDetails: RequestBody: %v", err, reqBody)
		log.Println(respErr)
		h.ErrorEventChannel <- respErr
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	//w.Header().Set("Content-Type", fmt.Sprint(len(challengeRequest.Challenge)))
	w.Header().Set("Content-Type", "text/plain")
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
		respErr := fmt.Errorf("handleSubscriptionEventNotification: cannot read request body: %w", err)
		log.Println(respErr)
		h.ErrorEventChannel <- respErr
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var event subscriptionEventNotificationRequest
	if err := json.Unmarshal(reqBody, &event); err != nil {
		respErr := fmt.Errorf("handleSubscriptionEventNotification: cannot unmarshal request body: %w\nDetails: RequestBody: %v", err, reqBody)
		h.ErrorEventChannel <- respErr
		log.Println(respErr)
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
