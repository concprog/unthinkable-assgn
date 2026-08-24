package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
)

func UpdateAgentLocation(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		}
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		err := repository.UpdateAgentLocation(c.Context(), pool, c.Params("id"), req.Latitude, req.Longitude)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "agent profile not found"})
		}
		return c.JSON(fiber.Map{"status": "location_updated"})
	}
}

// MyOrders — GET /api/agents/me/orders: the deliveries assigned to
// the signed-in agent, freshest first.
func MyOrders(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		tasks, err := repository.ListAssignedOrders(c.Context(), pool, userIDFrom(c))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list assigned orders"})
		}
		if tasks == nil {
			tasks = []repository.AgentTaskRow{}
		}
		return c.JSON(tasks)
	}
}

// SetAvailability — PATCH /api/agents/me/availability with
// { availability: AVAILABLE | BUSY | OFFLINE }.
func SetAvailability(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Availability string `json:"availability"`
		}
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		switch req.Availability {
		case "AVAILABLE", "BUSY", "OFFLINE":
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "availability must be AVAILABLE, BUSY or OFFLINE",
			})
		}

		profile, err := repository.UpdateAvailability(c.Context(), pool, userIDFrom(c), req.Availability)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "agent profile not found"})
		}
		return c.JSON(profile)
	}
}
