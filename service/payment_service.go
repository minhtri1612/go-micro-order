package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/sony/gobreaker"
)

// PaymentService handles payment-related operations
type PaymentService struct {
	baseURL       string
	client        *http.Client
	circuitBreaker *gobreaker.CircuitBreaker
}

// PaymentRequest represents a payment creation request
type PaymentRequest struct {
	OrderID    int     `json:"order_id"`
	CustomerID int     `json:"customer_id"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
}

// PaymentResponse represents a payment response
type PaymentResponse struct {
	Payment struct {
		ID                int     `json:"id"`
		OrderID           int     `json:"order_id"`
		CustomerID        int     `json:"customer_id"`
		Amount            float64 `json:"amount"`
		Currency          string  `json:"currency"`
		Status            string  `json:"status"`
		StripePaymentID   string  `json:"stripe_payment_id"`
		PaymentMethod     string  `json:"payment_method"`
		CreatedAt         string  `json:"created_at"`
		UpdatedAt         string  `json:"updated_at"`
	} `json:"payment"`
	ClientSecret string `json:"client_secret,omitempty"`
	Message      string `json:"message,omitempty"`
}

// NewPaymentService creates a new payment service instance
func NewPaymentService() *PaymentService {
	baseURL := getEnv("PAYMENT_SERVICE_URL", "http://payment:8084")
	
	// Circuit breaker settings
	settings := gobreaker.Settings{
		Name:        "PaymentService",
		MaxRequests: 3,
		Interval:    time.Second * 10,
		Timeout:     time.Second * 30,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 2
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			fmt.Printf("CircuitBreaker '%s' changed from '%s' to '%s'\n", name, from, to)
		},
	}

	return &PaymentService{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		circuitBreaker: gobreaker.NewCircuitBreaker(settings),
	}
}

// CreatePayment creates a payment intent for an order
func (ps *PaymentService) CreatePayment(orderID, customerID int, amount float64, currency string) (*PaymentResponse, error) {
	paymentReq := PaymentRequest{
		OrderID:    orderID,
		CustomerID: customerID,
		Amount:     amount,
		Currency:   currency,
	}

	jsonData, err := json.Marshal(paymentReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payment request: %w", err)
	}

	result, err := ps.circuitBreaker.Execute(func() (interface{}, error) {
		req, err := http.NewRequest("POST", ps.baseURL+"/payments", bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := ps.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			return nil, fmt.Errorf("payment service returned status: %d", resp.StatusCode)
		}

		var paymentResp PaymentResponse
		if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return &paymentResp, nil
	})

	if err != nil {
		return nil, fmt.Errorf("payment service circuit breaker: %w", err)
	}

	paymentResp, ok := result.(*PaymentResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from payment service")
	}

	return paymentResp, nil
}

// GetPaymentsByOrder retrieves payments for a specific order
func (ps *PaymentService) GetPaymentsByOrder(orderID int) ([]PaymentResponse, error) {
	url := fmt.Sprintf("%s/payments/order/%d", ps.baseURL, orderID)

	result, err := ps.circuitBreaker.Execute(func() (interface{}, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := ps.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to make request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("payment service returned status: %d", resp.StatusCode)
		}

		var payments []PaymentResponse
		if err := json.NewDecoder(resp.Body).Decode(&payments); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return payments, nil
	})

	if err != nil {
		return nil, fmt.Errorf("payment service circuit breaker: %w", err)
	}

	payments, ok := result.([]PaymentResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from payment service")
	}

	return payments, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}