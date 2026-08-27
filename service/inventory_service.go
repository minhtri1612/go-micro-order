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

// InventoryService is a client for the inventory service
type InventoryService struct {
	BaseURL    string
	HTTPClient *http.Client
	cb         *gobreaker.CircuitBreaker
}

// NewInventoryService creates a new inventory service client
func NewInventoryService() *InventoryService {
	baseURL := os.Getenv("INVENTORY_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://inventory:8082" // Kubernetes service DNS default
	}

	// Create circuit breaker
	cbConfig := resilience.DefaultConfig("inventory-service")
	cb := resilience.NewCircuitBreaker(cbConfig)

	return &InventoryService{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: time.Second * 10,
		},
		cb: cb,
	}
}

// CheckInventory checks if a product is available in inventory
func (is *InventoryService) CheckInventory(productID int, quantity int) (*model.InventoryResponse, error) {
	data := model.InventoryCheck{
		ProductID: productID,
		Quantity:  quantity,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal inventory check: %w", err)
	}

	// Use circuit breaker with retry
	result, err := resilience.ExecuteWithRetry(is.cb, func() (interface{}, error) {
		req, err := http.NewRequest("POST", fmt.Sprintf("%s/inventory/check", is.BaseURL), bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := is.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("inventory service request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("inventory service returned non-OK status: %d", resp.StatusCode)
		}

		var inventoryResponse model.InventoryResponse
		if err := json.NewDecoder(resp.Body).Decode(&inventoryResponse); err != nil {
			return nil, fmt.Errorf("failed to decode inventory response: %w", err)
		}

		return &inventoryResponse, nil
	}, 3) // Maximum 3 retries

	if err != nil {
		return nil, err
	}

	return result.(*model.InventoryResponse), nil
}

func (s *InventoryService) CheckAvailability(productID int, quantity int) (bool, error) {
	url := fmt.Sprintf("%s/check/%d?quantity=%d",
		s.BaseURL,
		productID,
		quantity)

	result, err := s.cb.Execute(func() (interface{}, error) {
		resp, err := s.HTTPClient.Get(url)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("inventory service returned status: %d", resp.StatusCode)
		}

		var response struct {
			Available bool `json:"available"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, err
		}

		return response.Available, nil
	})

	if err != nil {
		return false, err
	}

	return result.(bool), nil
}
