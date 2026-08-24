package main

import (
	"context"
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

	if err := db.Migrate(context.Background(), pool); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	store := db.NewLimiterStore(pool)

	app := fiber.New(fiber.Config{
		ServerHeader: "Fiber",
		AppName:      "lastmile-tracker-backend",
	})

	app.Use(limiter.New(limiter.Config{
		Max:        60,
		Expiration: 1 * time.Minute,
		Storage:    store,
	}))

	// Public auth endpoints — register/login issue the JWT that
	// RequireAuth verifies on everything below.
	app.Post("/api/auth/register", handlers.Register(pool))
	app.Post("/api/auth/login", handlers.Login(pool))
	app.Get("/api/auth/verify", handlers.VerifyEmail(pool))

	api := app.Group("/api", middleware.RequireAuth())
	api.Get("/auth/me", handlers.Me(pool))
	api.Post("/auth/send-verification", handlers.SendVerification(pool))

	orders := api.Group("/orders")
	orders.Post("/", handlers.CreateOrder(pool))
	orders.Get("/", handlers.ListMyOrders(pool))
	orders.Get("/:id<guid>", handlers.GetOrder(pool))
	orders.Post("/:id<guid>/assign", handlers.AssignOrder(pool))
	orders.Patch("/:id<guid>/status", handlers.UpdateOrderStatus(pool))
	orders.Post("/:id<guid>/reschedule", handlers.RescheduleOrder(pool))

	agents := api.Group("/agents", middleware.RequireRole("admin", "agent"))
	agents.Patch("/:id<guid>/location", handlers.UpdateAgentLocation(pool))
	me := agents.Group("/me")
	me.Get("/orders", handlers.MyOrders(pool))
	me.Patch("/availability", handlers.SetAvailability(pool))

	admin := api.Group("/admin", middleware.RequireRole("admin"))
	admin.Post("/zones", handlers.CreateZone(pool))
	admin.Get("/zones", handlers.ListZones(pool))
	admin.Get("/zones/:id<int>/areas", handlers.ListZoneAreas(pool))
	admin.Post("/zones/:id<int>/areas", handlers.AddZoneAreas(pool))
	admin.Delete("/zones/:id<int>/areas/:pincode", handlers.RemoveZoneArea(pool))
	admin.Get("/rate-cards", handlers.ListRateCards(pool))
	admin.Post("/rate-cards", handlers.CreateRateCard(pool))
	admin.Patch("/rate-cards/:id<int>", handlers.UpdateRateCard(pool))
	admin.Delete("/rate-cards/:id<int>", handlers.DeleteRateCard(pool))
	admin.Patch("/rate-cards/:id<int>/lanes", handlers.EditLane(pool))
	admin.Get("/orders", handlers.ListAdminOrders(pool))
	admin.Get("/orders/:id<guid>/nearby-agents", handlers.NearbyAgents(pool))

	app.Post("/webhooks/clerk", handlers.ClerkWebhook(pool))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(app.Listen(":"+port, fiber.ListenConfig{
		ListenerNetwork: "tcp",
	}))
}
