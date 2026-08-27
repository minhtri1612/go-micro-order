package unit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/minhtri1612/go-micro-order/controller"
	"github.com/minhtri1612/go-micro-order/model"
	"github.com/minhtri1612/go-micro-order/queue"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for unit testing
type MockInventoryService struct {
	mock.Mock
}

func (m *MockInventoryService) CheckAvailability(productID int, quantity int) (bool, error) {
	args := m.Called(productID, quantity)
	return args.Bool(0), args.Error(1)
}

type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) SendOrderNotification(orderID int) error {
	args := m.Called(orderID)
	return args.Error(0)
}

func (m *MockNotificationService) SendOrderStatusUpdate(orderID int, customerID int, status string) error {
	args := m.Called(orderID, customerID, status)
	return args.Error(0)
}

type MockOrderRepository struct {
	mock.Mock
}

func (m *MockOrderRepository) InsertOrder(order *model.Order) error {
	args := m.Called(order)
	return args.Error(0)
}

func (m *MockOrderRepository) GetOrderFromDB(orderID string) (*model.Order, error) {
	args := m.Called(orderID)
	order, ok := args.Get(0).(*model.Order)
	if !ok {
		return nil, args.Error(1)
	}
	return order, args.Error(1)
}

type MockMessageQueue struct {
	mock.Mock
}

func (m *MockMessageQueue) PublishMessage(config queue.Config, message interface{}) error {
	args := m.Called(config, message)
	return args.Error(0)
}

type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(key string, value interface{}) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *MockCache) Set(key string, value interface{}, expiration time.Duration) error {
	args := m.Called(key, value, expiration)
	return args.Error(0)
}

func (m *MockCache) GetOrSet(key string, value interface{}, expiration time.Duration, fn func() (interface{}, error)) error {
	args := m.Called(key, value, expiration, fn)
	return args.Error(0)
}

// setupTestEnvironment creates a test environment with mock dependencies
func setupTestEnvironment() (*gin.Engine, *MockOrderRepository, *MockInventoryService, *MockNotificationService, *MockMessageQueue, *MockCache) {
	// Setup Gin
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create mocks
	mockOrderRepo := new(MockOrderRepository)
	mockInventory := new(MockInventoryService)
	mockNotification := new(MockNotificationService)
	mockQueue := new(MockMessageQueue)
	mockCache := new(MockCache)

	// Create controller with mocks
	orderController := &controller.OrderController{
		OrderRepo:           mockOrderRepo,
		InventoryService:    mockInventory,
		NotificationService: mockNotification,
		Queue:               mockQueue,
		Cache:               mockCache,
	}

	// Setup routes
	router.POST("/orders", orderController.CreateOrder)
	router.GET("/orders/:id", orderController.GetOrder)

	return router, mockOrderRepo, mockInventory, mockNotification, mockQueue, mockCache
}

func TestCreateOrder_Success(t *testing.T) {
	// Setup
	router, mockOrderRepo, mockInventory, mockNotification, mockQueue, _ := setupTestEnvironment()

	// Prepare test data
	order := model.Order{
		ProductID:  1,
		CustomerID: 1,
		Quantity:   2,
	}

	// Set up mock expectations
	mockOrderRepo.On("InsertOrder", mock.AnythingOfType("*model.Order")).Return(nil)
	mockInventory.On("CheckAvailability", 1, 2).Return(true, nil)
	mockNotification.On("SendOrderNotification", mock.AnythingOfType("int")).Return(nil)
	mockQueue.On("PublishMessage", mock.AnythingOfType("queue.Config"), mock.Anything).Return(nil)

	// Create request
	orderJSON, _ := json.Marshal(order)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(orderJSON))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, w.Code)

	// Verify all mocks were called as expected
	mockOrderRepo.AssertExpectations(t)
	mockInventory.AssertExpectations(t)
	mockNotification.AssertExpectations(t)
	mockQueue.AssertExpectations(t)

	// Specifically verify that InsertOrder was called exactly once
	mockOrderRepo.AssertNumberOfCalls(t, "InsertOrder", 1)
	// Specifically verify that SendOrderNotification was called exactly once
	mockNotification.AssertNumberOfCalls(t, "SendOrderNotification", 1)
}

func TestCreateOrder_ProductNotAvailable(t *testing.T) {
	// Setup
	router, mockOrderRepo, mockInventory, mockNotification, mockQueue, _ := setupTestEnvironment()

	// Prepare test data
	order := model.Order{
		ProductID:  1,
		CustomerID: 1,
		Quantity:   100, // Large quantity that should not be available
	}

	// Set up mock expectations
	mockInventory.On("CheckAvailability", 1, 100).Return(false, nil)
	// Other mocks should not be called

	// Create request
	orderJSON, _ := json.Marshal(order)
	req := httptest.NewRequest("POST", "/orders", bytes.NewBuffer(orderJSON))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Verify mocks
	mockInventory.AssertExpectations(t)
	// Verify that other mocks were not called
	mockOrderRepo.AssertNotCalled(t, "InsertOrder")
	mockNotification.AssertNotCalled(t, "SendOrderNotification")
	mockQueue.AssertNotCalled(t, "PublishMessage")
}