package controller

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/minhtri1612/go-micro-order/cache"
	"github.com/minhtri1612/go-micro-order/metrics"
	"github.com/minhtri1612/go-micro-order/model"
	"github.com/minhtri1612/go-micro-order/queue"
	"github.com/minhtri1612/go-micro-order/service"
	"github.com/minhtri1612/go-micro-order/worker"

	"github.com/gin-gonic/gin"
)

// InventoryServiceInterface defines the interface for inventory service
type InventoryServiceInterface interface {
	CheckAvailability(productID int, quantity int) (bool, error)
}

// NotificationServiceInterface defines the interface for notification service
type NotificationServiceInterface interface {
	SendOrderNotification(orderID int) error
	SendOrderStatusUpdate(orderID int, customerID int, status string) error
}

// PaymentServiceInterface defines the interface for payment service
type PaymentServiceInterface interface {
	CreatePayment(orderID int, customerID int, amount float64, currency string) (*service.PaymentResponse, error)
}

// OrderRepository defines the interface for order database operations
type OrderRepository interface {
	InsertOrder(order *model.Order) error
	GetOrderFromDB(orderID string) (*model.Order, error)
}

// Cache defines the interface for cache operations
type Cache interface {
	Get(key string, value interface{}) error
	Set(key string, value interface{}, expiration time.Duration) error
	GetOrSet(key string, value interface{}, expiration time.Duration, fn func() (interface{}, error)) error
}

// MessageQueue defines the interface for message queue operations
type MessageQueue interface {
	PublishMessage(config queue.Config, message interface{}) error
}

// OrderController handles order-related requests
type OrderController struct {
	DB                  *sql.DB
	OrderRepo           OrderRepository
	Cache               Cache
	Queue               MessageQueue
	InventoryService    InventoryServiceInterface
	NotificationService NotificationServiceInterface
	PaymentService      PaymentServiceInterface
}

// DBOrderRepository implements OrderRepository interface using SQL database
type DBOrderRepository struct {
	DB *sql.DB
}

// InsertOrder inserts a new order into the database
func (r *DBOrderRepository) InsertOrder(order *model.Order) error {
	query := `
		INSERT INTO orders (customer_id, product_id, quantity, total_price, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	order.Status = "pending"
	order.CreatedAt = time.Now()

	return r.DB.QueryRow(
		query,
		order.CustomerID,
		order.ProductID,
		order.Quantity,
		order.TotalPrice,
		order.Status,
		order.CreatedAt,
	).Scan(&order.ID)
}

// GetOrderFromDB retrieves an order from the database by ID
func (r *DBOrderRepository) GetOrderFromDB(orderID string) (*model.Order, error) {
	var order model.Order
	query := `
		SELECT id, customer_id, product_id, quantity, total_price, status, created_at
		FROM orders
		WHERE id = $1`

	err := r.DB.QueryRow(query, orderID).Scan(
		&order.ID,
		&order.CustomerID,
		&order.ProductID,
		&order.Quantity,
		&order.TotalPrice,
		&order.Status,
		&order.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &order, nil
}

// RedisCache implements Cache interface using Redis
type RedisCache struct{}

// Get retrieves a value from cache
func (r *RedisCache) Get(key string, value interface{}) error { return cache.Get(key, value) }

// Set stores a value in cache with expiration
func (r *RedisCache) Set(key string, value interface{}, expiration time.Duration) error { return cache.Set(key, value, expiration) }

// GetOrSet retrieves value from cache or sets it if not exists
func (r *RedisCache) GetOrSet(key string, value interface{}, expiration time.Duration, fn func() (interface{}, error)) error {
	return cache.GetOrSet(key, value, expiration, fn)
}

// RabbitMQQueue implements MessageQueue interface using RabbitMQ
type RabbitMQQueue struct{}

// PublishMessage publishes a message to RabbitMQ
func (r *RabbitMQQueue) PublishMessage(config queue.Config, message interface{}) error {
	return queue.PublishMessage(config, message)
}

// NewOrderController creates a new order controller
func NewOrderController(db *sql.DB) *OrderController {
	return &OrderController{
		DB:                  db,
		OrderRepo:           &DBOrderRepository{DB: db},
		Cache:               &RedisCache{},
		Queue:               &RabbitMQQueue{},
		InventoryService:    service.NewInventoryService(),
		NotificationService: service.NewNotificationService(),
		PaymentService:      service.NewPaymentService(),
	}
}

// getProductPrice fetches a product's price from product-service
func getProductPrice(productID int) (float64, error) {
	baseURL := os.Getenv("PRODUCT_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://product:8080"
	}
	url := fmt.Sprintf("%s/products/%d", baseURL, productID)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch product: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("product service returned status: %d", resp.StatusCode)
	}

	var product struct {
		Price float64 `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		return 0, fmt.Errorf("failed to decode product response: %w", err)
	}
	return product.Price, nil
}

// checkInventoryAvailable checks availability via inventory-service POST /inventory/check
func checkInventoryAvailable(productID int, quantity int) (bool, error) {
	baseURL := os.Getenv("INVENTORY_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://inventory:8082"
	}

	payload := model.InventoryCheck{ProductID: productID, Quantity: quantity}
	body, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal inventory check: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/inventory/check", baseURL), bytes.NewBuffer(body))
	if err != nil {
		return false, fmt.Errorf("failed to create inventory request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("inventory service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("inventory service returned status: %d", resp.StatusCode)
	}

	var invResp model.InventoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&invResp); err != nil {
		return false, fmt.Errorf("failed to decode inventory response: %w", err)
	}

	// Backward/forward compatible with inventory-service payload changes.
	// Old key: "available", new key: "is_available".
	return invResp.Available || invResp.IsAvailable, nil
}

// CreateOrder handles creation of a new order
func (oc *OrderController) CreateOrder(c *gin.Context) {
	var order model.Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check inventory availability via correct endpoint
	available, err := checkInventoryAvailable(order.ProductID, order.Quantity)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to check inventory: " + err.Error()})
		return
	}
	if !available {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product not available in requested quantity"})
		return
	}

	// Compute total price from product-service if not provided
	if order.TotalPrice == 0 {
		price, err := getProductPrice(order.ProductID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch product price: " + err.Error()})
			return
		}
		order.TotalPrice = float64(order.Quantity) * price
	}

	// Insert order into database
	if oc.OrderRepo != nil {
		err = oc.OrderRepo.InsertOrder(&order)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order: " + err.Error()})
			return
		}
	} else {
		// For testing purposes, set a mock ID
		order.ID = 1
		order.Status = "pending"
		order.CreatedAt = time.Now()
	}

	// Publish order created event to message queue
	if oc.Queue != nil {
		orderMsg, _ := json.Marshal(order)
		if err := oc.Queue.PublishMessage(queue.Config{
			QueueName:    "orders",
			RoutingKey:   "order.created",
			ExchangeName: "orders",
		}, orderMsg); err != nil {
			log.Printf("Warning: Failed to publish order created event: %v\n", err)
		}
	}

	// Send notification using circuit breaker
	go func() {
		if err := oc.NotificationService.SendOrderNotification(order.ID); err != nil {
			log.Printf("Failed to send notification: %v\n", err)
		}
	}()

	c.JSON(http.StatusCreated, order)
}

// CreateOrderWithPayment handles creation of a new order with payment intent
func (oc *OrderController) CreateOrderWithPayment(c *gin.Context) {
	var orderWithPayment struct {
		model.Order
		Currency string `json:"currency" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&orderWithPayment); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check inventory availability using circuit breaker
	available, err := checkInventoryAvailable(orderWithPayment.ProductID, orderWithPayment.Quantity)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Failed to check inventory: " + err.Error()})
		return
	}
	if !available {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product not available in requested quantity"})
		return
	}

	// Compute total price from product-service if not provided
	if orderWithPayment.TotalPrice == 0 {
		price, err := getProductPrice(orderWithPayment.ProductID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch product price: " + err.Error()})
			return
		}
		orderWithPayment.TotalPrice = float64(orderWithPayment.Quantity) * price
	}

	// Insert order into database
	if oc.OrderRepo != nil {
		err = oc.OrderRepo.InsertOrder(&orderWithPayment.Order)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order: " + err.Error()})
			return
		}
	} else {
		// For testing purposes, set a mock ID
		orderWithPayment.Order.ID = 1
		orderWithPayment.Order.Status = "pending"
		orderWithPayment.Order.CreatedAt = time.Now()
	}

	// Create payment intent
	paymentResp, err := oc.PaymentService.CreatePayment(
		orderWithPayment.ID,
		orderWithPayment.CustomerID,
		orderWithPayment.TotalPrice,
		orderWithPayment.Currency,
	)
	if err != nil {
		log.Printf("Warning: Failed to create payment intent: %v\n", err)
		// Still return the order but indicate payment failed
		c.JSON(http.StatusCreated, gin.H{
			"order":         orderWithPayment.Order,
			"payment_error": "Failed to create payment intent: " + err.Error(),
		})
		return
	}

	// Publish order created event to message queue
	if oc.Queue != nil {
		orderMsg, _ := json.Marshal(orderWithPayment.Order)
		if err := oc.Queue.PublishMessage(queue.Config{
			QueueName:    "orders",
			RoutingKey:   "order.created",
			ExchangeName: "orders",
		}, orderMsg); err != nil {
			log.Printf("Warning: Failed to publish order created event: %v\n", err)
		}
	}

	// Send notification using circuit breaker
	go func() {
		if err := oc.NotificationService.SendOrderNotification(orderWithPayment.ID); err != nil {
			log.Printf("Failed to send notification: %v\n", err)
		}
	}()

	c.JSON(http.StatusCreated, gin.H{
		"order":   orderWithPayment.Order,
		"payment": paymentResp,
	})
}

// GetOrders returns all orders
func (oc *OrderController) GetOrders(c *gin.Context) {
	rows, err := oc.DB.Query("SELECT id, customer_id, product_id, quantity, total_price, status, created_at FROM orders")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var orders []model.Order
	for rows.Next() {
		var o model.Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.ProductID, &o.Quantity, &o.TotalPrice, &o.Status, &o.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		orders = append(orders, o)
	}

	c.JSON(http.StatusOK, orders)
}

// GetOrder returns a specific order by ID
func (oc *OrderController) GetOrder(c *gin.Context) {
	orderID := c.Param("id")

	// Try to get order from cache first
	var order model.Order
	cacheKey := "order:" + orderID
	
	// If cache is not available, get directly from database
	if oc.Cache == nil {
		if oc.OrderRepo != nil {
			order, err := oc.OrderRepo.GetOrderFromDB(orderID)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
					return
				}
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, order)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	
	err := oc.Cache.GetOrSet(cacheKey, &order, 30*time.Minute, func() (interface{}, error) {
		// If not in cache, get from database
		if oc.OrderRepo != nil {
			return oc.OrderRepo.GetOrderFromDB(orderID)
		}
		return nil, sql.ErrNoRows
	})

	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		// If cache backend is unavailable (e.g. REDIS_HOST not configured),
		// fall back to DB so rollout checks are not blocked by cache infra.
		if oc.OrderRepo != nil {
			order, dbErr := oc.OrderRepo.GetOrderFromDB(orderID)
			if dbErr == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
				return
			}
			if dbErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order: " + dbErr.Error()})
				return
			}
			c.JSON(http.StatusOK, order)
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}

// UpdateOrder updates an order
func (oc *OrderController) UpdateOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	// Get existing order to compare status change
	var existingOrder model.Order
	err = oc.DB.QueryRow("SELECT id, customer_id, product_id, quantity, total_price, status FROM orders WHERE id = $1", id).
		Scan(&existingOrder.ID, &existingOrder.CustomerID, &existingOrder.ProductID, &existingOrder.Quantity, &existingOrder.TotalPrice, &existingOrder.Status)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var updatedOrder model.Order
	if err := c.BindJSON(&updatedOrder); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := oc.DB.Exec(
		"UPDATE orders SET customer_id = $1, product_id = $2, quantity = $3, total_price = $4, status = $5 WHERE id = $6",
		updatedOrder.CustomerID, updatedOrder.ProductID, updatedOrder.Quantity, updatedOrder.TotalPrice, updatedOrder.Status, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// If status changed, send notification
	if existingOrder.Status != updatedOrder.Status {
		err = oc.NotificationService.SendOrderStatusUpdate(id, updatedOrder.CustomerID, updatedOrder.Status)
		if err != nil {
			// Log the error but continue (non-blocking)
			fmt.Printf("Failed to send status update notification: %v\n", err)
		}
	}

	updatedOrder.ID = id
	c.JSON(http.StatusOK, updatedOrder)
}

// DeleteOrder deletes an order
func (oc *OrderController) DeleteOrder(c *gin.Context) {
	id := c.Param("id")

	// Get the order first
	var order model.Order
	err := oc.DB.QueryRow("SELECT id, customer_id, product_id, quantity, total_price, status FROM orders WHERE id = $1", id).
		Scan(&order.ID, &order.CustomerID, &order.ProductID, &order.Quantity, &order.TotalPrice, &order.Status)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Delete the order
	result, err := oc.DB.Exec("DELETE FROM orders WHERE id = $1", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Send notification that order was deleted/cancelled
	err = oc.NotificationService.SendOrderStatusUpdate(order.ID, order.CustomerID, "cancelled")
	if err != nil {
		// Log the error but continue (non-blocking)
		fmt.Printf("Failed to send cancellation notification: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order deleted successfully"})
}

// UpdateOrderStatus updates only the status of an order
func (oc *OrderController) UpdateOrderStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	var statusUpdate struct {
		Status string `json:"status"`
	}
	if err := c.BindJSON(&statusUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get existing order to get customer ID
	var order model.Order
	err = oc.DB.QueryRow("SELECT id, customer_id, product_id, quantity, total_price, status FROM orders WHERE id = $1", id).
		Scan(&order.ID, &order.CustomerID, &order.ProductID, &order.Quantity, &order.TotalPrice, &order.Status)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update order status
	result, err := oc.DB.Exec("UPDATE orders SET status = $1 WHERE id = $2", statusUpdate.Status, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	metrics.OrderStatusUpdated.WithLabelValues(statusUpdate.Status).Inc()

	// Update active orders metric based on status
	if statusUpdate.Status == "completed" || statusUpdate.Status == "cancelled" {
		metrics.ActiveOrders.Dec()
	}

	// Send notification about status change
	err = oc.NotificationService.SendOrderStatusUpdate(id, order.CustomerID, statusUpdate.Status)
	if err != nil {
		// Log the error but continue (non-blocking)
		fmt.Printf("Failed to send status update notification: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Order status updated successfully",
		"order_id": id,
		"status":   statusUpdate.Status,
	})
}

// CreateBatchOrders handles creation of multiple orders in parallel
func (oc *OrderController) CreateBatchOrders(c *gin.Context) {
	var orders []model.Order
	if err := c.ShouldBindJSON(&orders); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Configure batch processing
	numWorkers := 10            // Số lượng worker xử lý song song
	timeout := 30 * time.Second // Thời gian timeout cho toàn bộ batch

	// Process orders in parallel using worker pool
	results := worker.ProcessBatch(orders, numWorkers, timeout)

	// Count successes and failures
	successful := 0
	failed := 0
	failedOrders := make([]map[string]interface{}, 0)

	for _, result := range results {
		if result.Error != nil {
			failed++
			failedOrders = append(failedOrders, map[string]interface{}{
				"order_id": result.OrderID,
				"error":    result.Error.Error(),
			})
		} else {
			successful++
		}
	}

	// Return summary
	c.JSON(http.StatusOK, gin.H{
		"total_orders":    len(orders),
		"successful":      successful,
		"failed":          failed,
		"failed_orders":   failedOrders,
		"processing_time": timeout,
	})
}
