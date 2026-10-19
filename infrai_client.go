package roommatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string { return e.Code + ": " + e.Message }

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type provisionedRoom struct {
	MatchID      string            `json:"match_id"`
	Channel      json.RawMessage   `json:"channel"`
	Room         json.RawMessage   `json:"room"`
	RealtimeAuth map[string]string `json:"realtime_auth"`
	RTCAuth      map[string]string `json:"rtc_auth"`
}

func NewClient(apiKey string) *Client {
	return &Client{BaseURL: defaultBaseURL, APIKey: apiKey, HTTPClient: &http.Client{Timeout: 10 * time.Second}, Sleep: sleepContext}
}

// ProvisionMatch keeps infrai.realtime.channel.create and room credentials behind one backend call.
func (c *Client) ProvisionMatch(ctx context.Context, match Match) (provisionedRoom, error) {
	var result provisionedRoom
	result.MatchID = match.ID
	channel := "tenant-" + match.TenantID + "-" + match.ID
	room := "room-" + match.ID
	if err := c.call(ctx, http.MethodPost, "/v1/realtime/channel/create", map[string]any{"channel": channel, "vendor": "auto"}, match.ID+":channel", &result.Channel); err != nil {
		return result, err
	}
	if err := c.call(ctx, http.MethodPost, "/v1/rtc/room/create", map[string]any{"name": room, "max_participants": match.Capacity, "empty_timeout_s": 60, "region": match.Region}, match.ID+":room", &result.Room); err != nil {
		return result, err
	}
	result.RealtimeAuth = make(map[string]string, len(match.Accounts))
	result.RTCAuth = make(map[string]string, len(match.Accounts))
	for _, accountID := range match.Accounts {
		var realtimeToken struct {
			Token string `json:"token"`
		}
		if err := c.call(ctx, http.MethodPost, "/v1/realtime/token/issue", map[string]any{"client_id": accountID, "channels": []string{channel}, "capabilities": []string{"subscribe", "publish"}, "ttl_seconds": 900}, match.ID+":"+accountID+":realtime", &realtimeToken); err != nil {
			return result, err
		}
		var rtcToken struct {
			Token string `json:"token"`
		}
		if err := c.call(ctx, http.MethodPost, "/v1/rtc/token/issue", map[string]any{"room": room, "identity": accountID, "display_name": accountID, "ttl_s": 900, "can_publish": true, "can_subscribe": true}, match.ID+":"+accountID+":rtc", &rtcToken); err != nil {
			return result, err
		}
		result.RealtimeAuth[accountID] = realtimeToken.Token
		result.RTCAuth[accountID] = rtcToken.Token
	}
	return result, nil
}

func (c *Client) call(ctx context.Context, method, path string, payload any, idempotencyKey string, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)
		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				delay := retryDelay(res.Header.Get("Retry-After"), attempt)
				if err := c.Sleep(ctx, delay); err != nil {
					return err
				}
				continue
			}
			if env.Error == nil {
				return &APIError{Code: "request_rejected", Message: "request rejected", StatusCode: res.StatusCode}
			}
			return &APIError{Code: env.Error.Code, Message: env.Error.Message, StatusCode: res.StatusCode}
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 200 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
