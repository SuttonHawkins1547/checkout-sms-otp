package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSendOTPRequestAndRetry(t *testing.T) {
	requests := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sms/otp" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "otp:ord-204" {
			t.Fatal("missing auth or idempotency header")
		}
		if requests == 1 {
			return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"0"}}, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error":"rate limited"}`))}, nil
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"ok":true,"data":{},"metadata":{}}`))}, nil
	})}
	clientUnderTest := &InfraiSMS{APIKey: "test-key", BaseURL: "https://example.test", HTTPClient: client, Sleep: func(context.Context, time.Duration) error { return nil }}
	if err := clientUnderTest.SendOTP(context.Background(), "+15551234567", "otp:ord-204"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}
