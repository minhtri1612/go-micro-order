package model

import "time"

// Order represents an order entity
type Order struct {
	ID         int       `json:"id"`
	CustomerID int       `json:"customer_id"`
	ProductID  int       `json:"product_id"`
	Quantity   int       `json:"quantity"`
	TotalPrice float64   `json:"total_price"`
	Status     string    `json:"status"` // pending, processing, shipped, delivered, cancelled
	CreatedAt  time.Time `json:"created_at"`
}

// InventoryCheck is used to check inventory availability
type InventoryCheck struct {
	ProductID int `json:"product_id"`
	Quantity  int `json:"quantity"`
}

// InventoryResponse is the response from inventory service
type InventoryResponse struct {
	Available   bool   `json:"available"`
	IsAvailable bool   `json:"is_available"`
	Message     string `json:"message,omitempty"`
}

// OrderStatusUpdate is used to notify about order status updates
type OrderStatusUpdate struct {
	OrderID    int    `json:"order_id"`
	CustomerID int    `json:"customer_id"`
	Status     string `json:"status"`
}
