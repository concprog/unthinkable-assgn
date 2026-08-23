package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
	"lastmile-tracker/backend/services"
)

type CreateOrderRequest struct {
	CustomerID    string     `json:"customer_id"`
	PickupAddress AddressDTO `json:"pickup_address"`
	DropAddress   AddressDTO `json:"drop_address"`
	Package       PackageDTO `json:"package"`
	OrderType     string     `json:"order_type"`
	PaymentType   string     `json:"payment_type"`
	OrderValue    float64    `json:"order_value"`
}

type AddressDTO struct {
	Line1   string  `json:"line1"`
	Line2   string  `json:"line2"`
	City    string  `json:"city"`
	State   string  `json:"state"`
	Pincode string  `json:"pincode"`
	Lat     float64 `json:"latitude"`
	Lng     float64 `json:"longitude"`
}

type PackageDTO struct {
	LengthCM       float64 `json:"length_cm"`
	BreadthCM      float64 `json:"breadth_cm"`
	HeightCM       float64 `json:"height_cm"`
	ActualWeightKG float64 `json:"actual_weight_kg"`
}

func CreateOrder(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req CreateOrderRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		result, err := services.CreateOrder(c.Context(), pool, &req)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	}
}

func GetOrder(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		order, err := repository.GetOrderByID(c.Context(), pool, c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
		}
		return c.JSON(order)
	}
}

func AssignOrder(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Mode string `json:"mode"`
		}
		if err := c.Bind().Body(&req); err != nil || req.Mode != "AUTO" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mode must be AUTO"})
		}

		result, err := services.AssignAgent(c.Context(), pool, c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	}
}

func UpdateOrderStatus(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Status    string `json:"status"`
			ActorID   string `json:"actor_id"`
			ActorType string `json:"actor_type"`
		}
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		result, err := services.UpdateStatus(c.Context(), pool, c.Params("id"), &req)
		if err != nil {
			return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	}
}

func RescheduleOrder(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			RequestedDate string `json:"requested_date"`
		}
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		result, err := services.Reschedule(c.Context(), pool, c.Params("id"), req.RequestedDate)
		if err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(result)
	}
}
