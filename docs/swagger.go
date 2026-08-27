package docs

// @title Order Service API
// @version 1.0
// @description This is the Order Service API documentation.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8081
// @BasePath /
// @schemes http

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

// @tag.name orders
// @tag.description Order management endpoints

// CreateOrder godoc
// @Summary Create a new order
// @Description Create a new order with the provided details
// @Tags orders
// @Accept json
// @Produce json
// @Param order body model.Order true "Order object"
// @Success 201 {object} model.Order
// @Failure 400 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /orders [post]
func CreateOrderDoc() {}

// GetOrder godoc
// @Summary Get order by ID
// @Description Get order details by its ID
// @Tags orders
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} model.Order
// @Failure 404 {object} map[string]string
// @Router /orders/{id} [get]
func GetOrderDoc() {}

// CreateBatchOrders godoc
// @Summary Create multiple orders
// @Description Create multiple orders in batch
// @Tags orders
// @Accept json
// @Produce json
// @Param orders body []model.Order true "Array of orders"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /orders/batch [post]
func CreateBatchOrdersDoc() {}

// UpdateOrder godoc
// @Summary Update an order
// @Description Update an existing order
// @Tags orders
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Param order body model.Order true "Updated order object"
// @Success 200 {object} model.Order
// @Failure 404 {object} map[string]string
// @Router /orders/{id} [put]
func UpdateOrderDoc() {}

// DeleteOrder godoc
// @Summary Delete an order
// @Description Delete an order by its ID
// @Tags orders
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /orders/{id} [delete]
func DeleteOrderDoc() {}

// UpdateOrderStatus godoc
// @Summary Update order status
// @Description Update the status of an existing order
// @Tags orders
// @Accept json
// @Produce json
// @Param id path int true "Order ID"
// @Param status body map[string]string true "Status object"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /orders/{id}/status [patch]
func UpdateOrderStatusDoc() {}
