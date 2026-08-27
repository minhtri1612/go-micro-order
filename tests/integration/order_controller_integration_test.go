package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/minhtri1612/go-micro-order/cache"
	"github.com/minhtri1612/go-micro-order/controller"
	"github.com/minhtri1612/go-micro-order/db"
	"github.com/minhtri1612/go-micro-order/model"
	"github.com/minhtri1612/go-micro-order/queue"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// setupIntegrationTestEnvironment creates a test environment with real dependencies
func setupIntegrationTestEnvironment(t *testing.T) (*gin.Engine, *sql.DB, func()) {
	// Check if we should skip integration tests
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration test")
	}

	// Setup database
	database, err := func() (*sql.DB, error) {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Database connection failed: %v\n", r)
			}
		}()
		return db.GetDB(), nil
	}()
	if database == nil || err != nil {
		t.Skipf("Failed to initialize database: %v", err)
	}

	// Setup Redis via cache package
	if err := cache.InitRedis(); err != nil {
		t.Skipf("Failed to initialize Redis: %v", err)
	}

	// Setup RabbitMQ
	if err := queue.InitRabbitMQ(); err != nil {
		t.Skipf("Failed to initialize RabbitMQ: %v", err)
	}

	// Create controller with real dependencies
	orderController := controller.NewOrderController(database)

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/orders", orderController.CreateOrder)
	router.GET("/orders/:id", orderController.GetOrder)

	// Cleanup function
	cleanup := func() {
		// Clean up test data
		if database != nil {
			database.Exec("DELETE FROM orders WHERE customer_id = 999")
			database.Close()
		}

		// Clean up Redis
		cache.Flush()
		cache.Close()

		// Close RabbitMQ
		queue.Close()
	}

	return router, database, cleanup
}

func TestCreateOrderIntegration_Success(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration test")
	}

	router, _, cleanup := setupIntegrationTestEnvironment(t)
	defer cleanup()

	// Prepare test data
	order := model.Order{
		ProductID:  1,
		CustomerID: 999, // Use a test customer ID
		Quantity:   1,
	}

	// Create request
	orderJSON, _ := json.Marshal(order)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(orderJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, w.Code, "Response body: %s", w.Body.String())

	// Parse response
	var createdOrder model.Order
	err := json.Unmarshal(w.Body.Bytes(), &createdOrder)
	assert.NoError(t, err)
	assert.NotZero(t, createdOrder.ID)
	assert.Equal(t, "pending", createdOrder.Status)

	// Test that order is cached
	cacheKey := "order:" + strconv.Itoa(int(createdOrder.ID))
	var cachedOrder model.Order
	err = cache.Get(cacheKey, &cachedOrder)
	assert.NoError(t, err, "Cache lookup failed")
	assert.Equal(t, createdOrder.ID, cachedOrder.ID)
}

func TestGetOrderIntegration_Success(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
		t.Skip("Skipping integration test")
	}

	router, _,  cleanup := setupIntegrationTestEnvironment(t)
	defer cleanup()

	// First create an order
	order := model.Order{
		ProductID:  1,
		CustomerID: 999,
		Quantity:   1,
	}
	orderJSON, _ := json.Marshal(order)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(orderJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "Response body: %s", w.Body.String())

	var createdOrder model.Order
	err := json.Unmarshal(w.Body.Bytes(), &createdOrder)
	assert.NoError(t, err)

	// Now get the order
	req = httptest.NewRequest("GET", "/orders/"+strconv.Itoa(int(createdOrder.ID)), nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "Response body: %s", w.Body.String())

	var retrievedOrder model.Order
	err = json.Unmarshal(w.Body.Bytes(), &retrievedOrder)
	assert.NoError(t, err)
	assert.Equal(t, createdOrder.ID, retrievedOrder.ID)
}
