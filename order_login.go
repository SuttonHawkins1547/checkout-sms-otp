package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Checkout struct {
	OrderID    string `json:"order_id"`
	Phone      string `json:"phone"`
	TotalCents int    `json:"total_cents"`
	Status     string `json:"status"`
}

type Receipt struct {
	OrderID    string `json:"order_id"`
	TotalCents int    `json:"total_cents"`
	Status     string `json:"status"`
}

type CustomerUpdate struct {
	OrderID string `json:"order_id"`
	Message string `json:"message"`
}

type VerificationResult struct {
	Checkout Checkout       `json:"checkout"`
	Receipt  Receipt        `json:"receipt"`
	Update   CustomerUpdate `json:"customer_update"`
}

type OrderLogin struct {
	sms    SMSGateway
	mu     sync.Mutex
	orders map[string]Checkout
}

func NewOrderLogin(sms SMSGateway) *OrderLogin {
	return &OrderLogin{sms: sms, orders: make(map[string]Checkout)}
}

func (s *OrderLogin) Start(ctx context.Context, checkout Checkout) (Checkout, error) {
	if checkout.OrderID == "" || checkout.Phone == "" || checkout.TotalCents <= 0 {
		return Checkout{}, errors.New("order_id, phone, and positive total_cents are required")
	}
	checkout.Status = "awaiting_phone"
	if err := s.sms.SendOTP(ctx, checkout.Phone, "otp:"+checkout.OrderID); err != nil {
		return Checkout{}, err
	}
	s.mu.Lock()
	s.orders[checkout.OrderID] = checkout
	s.mu.Unlock()
	return checkout, nil
}

func (s *OrderLogin) Verify(ctx context.Context, orderID, code string) (VerificationResult, error) {
	s.mu.Lock()
	checkout, found := s.orders[orderID]
	s.mu.Unlock()
	if !found {
		return VerificationResult{}, errors.New("order not found")
	}
	if code == "" {
		return VerificationResult{}, errors.New("code is required")
	}
	if err := s.sms.VerifyOTP(ctx, checkout.Phone, code, "verify:"+orderID); err != nil {
		return VerificationResult{}, err
	}

	checkout.Status = "fulfilling"
	s.mu.Lock()
	s.orders[orderID] = checkout
	s.mu.Unlock()
	return VerificationResult{
		Checkout: checkout,
		Receipt:  Receipt{OrderID: orderID, TotalCents: checkout.TotalCents, Status: "issued"},
		Update:   CustomerUpdate{OrderID: orderID, Message: fmt.Sprintf("Order %s is confirmed and entering fulfillment.", orderID)},
	}, nil
}
