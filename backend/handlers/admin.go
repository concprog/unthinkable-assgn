package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
)

type CreateZoneRequest struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Pincodes    []repository.PincodeInput `json:"pincodes"`
}

func CreateZone(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req CreateZoneRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		zoneID, err := repository.CreateZone(c.Context(), pool, &req.Name, &req.Description, req.Pincodes)
		if err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"zone_id": zoneID})
	}
}
