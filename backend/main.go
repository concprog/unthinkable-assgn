package main

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"

	"lastmile-tracker/backend/db"
	"lastmile-tracker/backend/handlers"
	"lastmile-tracker/backend/middleware"
)

func main() {
	pool := db.Connect()

	store := db.NewLimiterStore(pool)

	app := fiber.New()

	app.Use(limiter.New(limiter.Config{
		Max:        60,
		Expiration: 1 * time.Minute,
		Storage:    store,
	}))

	api := app.Group("/api", middleware.ClerkAuth())

	orders := api.Group("/orders")
	orders.Post("/", handlers.CreateOrder(pool))
	orders.Get("/:id<guid>", handlers.GetOrder(pool))
	orders.Post("/:id<guid>/assign", handlers.AssignOrder(pool))
	orders.Patch("/:id<guid>/status", handlers.UpdateOrderStatus(pool))
	orders.Post("/:id<guid>/reschedule", handlers.RescheduleOrder(pool))

	agents := api.Group("/agents", middleware.RequireRole("admin", "agent"))
	agents.Patch("/:id<guid>/location", handlers.UpdateAgentLocation(pool))

	admin := api.Group("/admin", middleware.RequireRole("admin"))
	admin.Post("/zones", handlers.CreateZone(pool))

	app.Post("/webhooks/clerk", handlers.ClerkWebhook(pool))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(app.Listen(":" + port))
}
