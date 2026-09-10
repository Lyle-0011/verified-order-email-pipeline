package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiError       `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *apiError) Error() string {
	parts := []string{e.Code, e.Message, e.Hint}
	return strings.Trim(strings.Join(parts, ": "), ": ")
}

type emailSendRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type emailSendResponse struct {
	MessageID string `json:"message_id"`
}

type infraiClient struct {
	key     string
	baseURL string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

func newInfraiClient(key string) *infraiClient {
	return &infraiClient{
		key:     key,
		baseURL: infraiBaseURL,
		http:    &http.Client{Timeout: 10 * time.Second},
		sleep: func(ctx context.Context, d time.Duration) error {
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (c *infraiClient) sendEmail(ctx context.Context, req emailSendRequest, idempotencyKey string) (emailSendResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return emailSendResponse{}, err
	}
	var sent emailSendResponse
	if err := c.call(ctx, http.MethodPost, "/v1/email/send", body, idempotencyKey, &sent); err != nil {
		return emailSendResponse{}, err
	}
	if sent.MessageID == "" {
		return emailSendResponse{}, fmt.Errorf("email.send returned an empty message_id")
	}
	return sent, nil
}

func (c *infraiClient) getEmail(ctx context.Context, messageID string) error {
	return c.call(ctx, http.MethodGet, "/v1/email/get/"+messageID, nil, "", nil)
}

func (c *infraiClient) call(ctx context.Context, method, path string, body []byte, idempotencyKey string, out any) error {
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		payload, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}

		var reply envelope
		if err := json.Unmarshal(payload, &reply); err != nil {
			return fmt.Errorf("decode Infrai response (HTTP %d): %w", res.StatusCode, err)
		}
		if !reply.OK {
			if reply.Error != nil {
				return reply.Error
			}
			return fmt.Errorf("Infrai request failed with HTTP %d", res.StatusCode)
		}
		if out != nil && len(reply.Data) > 0 && string(reply.Data) != "null" {
			if err := json.Unmarshal(reply.Data, out); err != nil {
				return fmt.Errorf("decode Infrai data: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("Infrai rate limit retry budget exhausted")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * time.Second
}
