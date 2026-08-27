# Test Structure

This directory contains tests for the order service, organized into two main categories:

## Unit Tests (`/unit`)

Unit tests focus on testing the controller logic in isolation using mocks for all dependencies:
- Database operations (OrderRepository)
- External service calls (InventoryService, NotificationService)
- Cache operations (Cache)
- Message queue operations (MessageQueue)

Unit tests use the testify/mock library to create mock implementations of all dependencies.

### Running Unit Tests
```bash
go test ./tests/unit/... -v
```

## Integration Tests (`/integration`)

Integration tests verify the complete flow using real dependencies:
- Real PostgreSQL database
- Real Redis cache
- Real RabbitMQ message broker

These tests require the respective services to be running.

### Running Integration Tests
```bash
# Run all integration tests
go test ./tests/integration/... -v

# Skip integration tests (for CI environments without services)
SKIP_INTEGRATION_TESTS=true go test ./tests/... -v
```

## Test Structure Guidelines

1. **Unit Tests**: Test business logic only, mock all external dependencies
2. **Integration Tests**: Test the complete flow with real services
3. **Test Data**: Use isolated test data that doesn't interfere with production
4. **Cleanup**: Always clean up test data after tests complete
5. **Table-Driven Tests**: Use table-driven tests for multiple test cases
6. **Assertions**: Use testify/assert for clear test assertions

## Example Test Patterns

### Unit Test with Mocks
```go
func TestCreateOrder_Success(t *testing.T) {
    // Setup mocks
    mockRepo := new(MockOrderRepository)
    mockInventory := new(MockInventoryService)
    
    // Set expectations
    mockRepo.On("InsertOrder", mock.Anything).Return(nil)
    mockInventory.On("CheckAvailability", 1, 2).Return(true, nil)
    
    // Test
    // ...
    
    // Verify
    mockRepo.AssertExpectations(t)
    mockInventory.AssertExpectations(t)
}
```

### Integration Test with Real Services
```go
func TestCreateOrderIntegration_Success(t *testing.T) {
    // Skip in CI
    if os.Getenv("SKIP_INTEGRATION_TESTS") == "true" {
        t.Skip("Skipping integration test")
    }
    
    // Setup real services
    db := initTestDB()
    redis := initTestRedis()
    defer cleanup()
    
    // Test
    // ...
}
```