package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/minhtri1612/go-micro-order/cache"
	"github.com/minhtri1612/go-micro-order/controller"
	"github.com/minhtri1612/go-micro-order/model"
	"github.com/minhtri1612/go-micro-order/queue"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func setupTestEnvironment(t *testing.T) (*gin.Engine, *redis.Client, func()) {
	// Setup Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use different DB for testing
	})

	// Setup RabbitMQ
	err := queue.InitRabbitMQ()
	if err != nil {
		t.Fatalf("Failed to initialize RabbitMQ: %v", err)
	}

	// Setup router and controller
	router := gin.New()
	orderController := controller.NewOrderController(nil) // Pass test DB here
	router.POST("/orders", orderController.CreateOrder)
	router.GET("/orders/:id", orderController.GetOrder)
	router.POST("/orders/batch", orderController.CreateBatchOrders)

	// Return cleanup function
	cleanup := func() {
		if redisClient != nil {
			redisClient.FlushDB(context.Background())
			redisClient.Close()
		}
		queue.Close()
	}

	return router, redisClient, cleanup
}

func TestOrderFlowIntegration(t *testing.T) {
	router, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test creating an order
	order := model.Order{
		ProductID:  1,
		CustomerID: 1,
		Quantity:   2,
	}
	orderJSON, _ := json.Marshal(order)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(orderJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var createdOrder model.Order
	err := json.Unmarshal(w.Body.Bytes(), &createdOrder)
	assert.NoError(t, err)
	assert.NotZero(t, createdOrder.ID)

	// Test retrieving the order from cache
	cacheKey := fmt.Sprintf("order:%d", createdOrder.ID)
	var cachedOrder model.Order
	err = cache.Get(cacheKey, &cachedOrder)
	assert.NoError(t, err)
	assert.Equal(t, createdOrder.ID, cachedOrder.ID)

	// Test batch order creation
	orders := []model.Order{
		{ProductID: 1, CustomerID: 1, Quantity: 1},
		{ProductID: 2, CustomerID: 1, Quantity: 2},
	}
	batchJSON, _ := json.Marshal(orders)

	w = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/orders/batch", bytes.NewBuffer(batchJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), response["total_orders"])
}

func TestCacheIntegration(t *testing.T) {
	router, redisClient, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create test data
	testOrder := model.Order{
		ID:        1,
		ProductID: 1,
		Status:    "pending",
	}

	// Set in cache
	cacheKey := "order:1"
	err := cache.Set(cacheKey, testOrder, 1*time.Minute)
	assert.NoError(t, err)

	// Test retrieving from cache via API
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/orders/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response model.Order
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, testOrder.ID, response.ID)

	// Test cache expiration
	redisClient.Del(context.Background(), cacheKey)
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/orders/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMessageQueueIntegration(t *testing.T) {
	router, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Setup test queue
	testQueue := queue.Config{
		QueueName:    "test_orders",
		RoutingKey:   "test.order.created",
		ExchangeName: "test_orders",
		ExchangeType: "topic",
	}
	err := queue.DeclareQueue(testQueue)
	assert.NoError(t, err)

	// Create a channel to receive messages
	messages := make(chan []byte)
	err = queue.ConsumeMessages(testQueue, func(msg []byte) error {
		messages <- msg
		return nil
	})
	assert.NoError(t, err)

	// Create an order
	order := model.Order{
		ProductID:  1,
		CustomerID: 1,
		Quantity:   2,
	}
	orderJSON, _ := json.Marshal(order)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(orderJSON))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	// Wait for message
	select {
	case msg := <-messages:
		var receivedOrder model.Order
		err := json.Unmarshal(msg, &receivedOrder)
		assert.NoError(t, err)
		assert.Equal(t, order.ProductID, receivedOrder.ProductID)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}
