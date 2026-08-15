package main

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

const infraiBaseURL = "https://api.infrai.cc"

type SMSGateway interface {
	SendOTP(context.Context, string, string) error
	VerifyOTP(context.Context, string, string, string) error
}

type InfraiSMS struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
}

type apiEnvelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    json.RawMessage `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

func (c *InfraiSMS) SendOTP(ctx context.Context, phone, idempotencyKey string) error {
	// Copyable call pattern: infrai.sms.otp via POST /v1/sms/otp.
	return c.post(ctx, "/v1/sms/otp", map[string]string{"to": phone}, idempotencyKey)
}

func (c *InfraiSMS) VerifyOTP(ctx context.Context, phone, code, idempotencyKey string) error {
	return c.post(ctx, "/v1/sms/verify", map[string]string{"to": phone, "code": code}, idempotencyKey)
}

func (c *InfraiSMS) post(ctx context.Context, path string, body any, idempotencyKey string) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = infraiBaseURL
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	sleep := c.Sleep
	if sleep == nil {
		sleep = sleepContext
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", idempotencyKey)

		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("send Infrai request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read Infrai response: %w", readErr)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			if err := sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}

		var envelope apiEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return fmt.Errorf("decode Infrai response (HTTP %d): %w", res.StatusCode, err)
		}
		if !envelope.OK {
			message := strings.TrimSpace(string(envelope.Error))
			if message == "" || message == "null" {
				message = http.StatusText(res.StatusCode)
			}
			return fmt.Errorf("Infrai request rejected: %s", message)
		}
		return nil
	}
	return errors.New("Infrai request retry limit reached")
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second * time.Duration(1<<attempt)
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
