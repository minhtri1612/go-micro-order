package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/minhtri1612/go-micro-order/model"
	"github.com/minhtri1612/go-micro-order/resilience"

	"github.com/sony/gobreaker"
)

// NotificationService is a client for the notification service
type NotificationService struct {
	BaseURL    string
	HTTPClient *http.Client
	cb         *gobreaker.CircuitBreaker
}

// NewNotificationService creates a new notification service client
func NewNotificationService() *NotificationService {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://noti:8083" // Kubernetes service DNS default
	}

	// Create circuit breaker
	cbConfig := resilience.DefaultConfig("notification-service")
	cb := resilience.NewCircuitBreaker(cbConfig)

	return &NotificationService{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: time.Second * 10,
		},
		cb: cb,
	}
}

// SendOrderNotification sends an order notification to the notification service
func (ns *NotificationService) SendOrderNotification(orderID int) error {
	url := fmt.Sprintf("%s/notify/order/%d", ns.BaseURL, orderID)

	notification := struct {
		OrderID int    `json:"order_id"`
		Type    string `json:"type"`
	}{
		OrderID: orderID,
		Type:    "order_created",
	}

	body, err := json.Marshal(notification)
	if err != nil {
		return err
	}

	_, err = ns.cb.Execute(func() (interface{}, error) {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := ns.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("notification service request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("notification service returned status: %d", resp.StatusCode)
		}

		return nil, nil
	})

	return err
}

// SendOrderStatusUpdate sends an order status update to the notification service
func (ns *NotificationService) SendOrderStatusUpdate(orderID int, customerID int, status string) error {
	data := model.OrderStatusUpdate{
		OrderID:    orderID,
		CustomerID: customerID,
		Status:     status,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal order status update: %w", err)
	}

	// Use circuit breaker with retry
	_, err = resilience.ExecuteWithRetry(ns.cb, func() (interface{}, error) {
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/notify/status", ns.BaseURL), bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := ns.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("notification service request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("notification service returned status: %d", resp.StatusCode)
		}

		return nil, nil
	}, 3) // Maximum 3 retries

	return err
}
