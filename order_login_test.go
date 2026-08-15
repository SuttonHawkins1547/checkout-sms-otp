package main

import (
	"context"
	"errors"
	"testing"
)

type fakeSMS struct{ verifyErr error }

func (f *fakeSMS) SendOTP(context.Context, string, string) error { return nil }
func (f *fakeSMS) VerifyOTP(context.Context, string, string, string) error {
	return f.verifyErr
}

func TestVerificationControlsFulfillment(t *testing.T) {
	tests := []struct {
		name       string
		verifyErr  error
		wantStatus string
		wantErr    bool
	}{
		{name: "accepted code releases order", wantStatus: "fulfilling"},
		{name: "rejected code keeps order waiting", verifyErr: errors.New("code rejected"), wantStatus: "awaiting_phone", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewOrderLogin(&fakeSMS{verifyErr: tt.verifyErr})
			_, err := service.Start(context.Background(), Checkout{OrderID: "ord-204", Phone: "+15551234567", TotalCents: 4590})
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.Verify(context.Background(), "ord-204", "123456")
			if (err != nil) != tt.wantErr {
				t.Fatalf("Verify() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				service.mu.Lock()
				result.Checkout = service.orders["ord-204"]
				service.mu.Unlock()
			}
			if result.Checkout.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", result.Checkout.Status, tt.wantStatus)
			}
		})
	}
}
