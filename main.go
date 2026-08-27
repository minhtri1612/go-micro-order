package main

import (
	"log"

	"github.com/minhtri1612/go-micro-order/cache"
	"github.com/minhtri1612/go-micro-order/controller"
	"github.com/minhtri1612/go-micro-order/db"
	"github.com/minhtri1612/go-micro-order/queue"
	"github.com/minhtri1612/go-micro-order/routes"

	"github.com/gin-gonic/gin"
	"github.com/penglongli/gin-metrics/ginmetrics"
)

func main() {
	// Initialize database connection
	database := db.GetDB()
	defer database.Close()

	// Initialize database schema
	db.InitSchema(database)

	// Initialize Redis
	if err := cache.InitRedis(); err != nil {
		log.Printf("Warning: Failed to initialize Redis: %v\n", err)
	}

	// Initialize RabbitMQ
	if err := queue.InitRabbitMQ(); err != nil {
		log.Printf("Warning: Failed to initialize RabbitMQ: %v\n", err)
	}
	defer queue.Close()

	// Declare queues
	orderQueue := queue.Config{
		QueueName:    "orders",
		RoutingKey:   "order.#",
		ExchangeName: "orders",
		ExchangeType: "topic",
	}
	if err := queue.DeclareQueue(orderQueue); err != nil {
		log.Printf("Warning: Failed to declare order queue: %v\n", err)
	}

	// Create order controller
	orderController := controller.NewOrderController(database)

	// Initialize router
	router := gin.Default()

	// HTTP metrics middleware
	m := ginmetrics.GetMonitor()
	m.SetMetricPath("/metrics")
	m.SetSlowTime(2)
	m.SetDuration([]float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5})
	m.Use(router)


	// Setup routes
	routes.SetupRoutes(router, orderController)

	// Start server
	log.Println("Order Service starting on port 8081...")
	if err := router.Run(":8081"); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}
