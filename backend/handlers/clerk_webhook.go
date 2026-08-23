package handlers

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
)

type ClerkEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func ClerkWebhook(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var event ClerkEvent
		if err := json.Unmarshal(c.Body(), &event); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		var err error
		switch event.Type {
		case "user.created", "user.updated":
			err = repository.UpsertUserFromClerk(c.Context(), pool, event.Data)
		case "user.deleted":
			err = repository.DeactivateUserFromClerk(c.Context(), pool, event.Data)
		default:
			return c.JSON(fiber.Map{"status": "ignored", "event": event.Type})
		}

		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"status": "synced"})
	}
}
