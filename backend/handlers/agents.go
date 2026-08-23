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
