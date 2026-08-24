package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
	"lastmile-tracker/backend/services"
)

type CreateOrderRequest struct {
	PickupAddress services.OrderAddressInput `json:"pickup_address"`
	DropAddress   services.OrderAddressInput `json:"drop_address"`
	Package       services.PackageInput      `json:"package"`
	OrderType     string                     `json:"order_type"`
	PaymentType   string                     `json:"payment_type"`
	OrderValue    float64                    `json:"order_value"`
}

func userIDFrom(c fiber.Ctx) string {
	id, _ := c.Locals("user_id").(string)
	return id
}

func CreateOrder(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req CreateOrderRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		result, err := services.CreateOrder(c.Context(), pool, userIDFrom(c), &services.CreateOrderInput{
			PickupAddress: req.PickupAddress,
			DropAddress:   req.DropAddress,
			Package:       req.Package,
			OrderType:     req.OrderType,
			PaymentType:   req.PaymentType,
			OrderValue:    req.OrderValue,
		})
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	}
}

// ListMyOrders — GET /api/orders: everything the signed-in user
// placed or had created on their behalf.
func ListMyOrders(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		orders, err := repository.ListOrdersForUser(c.Context(), pool, userIDFrom(c))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list orders"})
		}
		if orders == nil {
			orders = []repository.OrderSummaryRow{}
		}
		return c.JSON(orders)
	}
}

// GetOrder — one query serves both the customer tracking page
// (charge breakdown + history timeline) and the agent action page
// (drop address, customer name, COD info).
func GetOrder(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		order, err := repository.GetOrderDetail(c.Context(), pool, c.Params("id"))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
		}

		history, err := repository.GetOrderHistory(c.Context(), pool, order.ID)
		if err != nil {
			history = []repository.StatusHistoryRow{}
		}

		var assignedAgentID *string
		if order.AssignedAgentID != nil && *order.AssignedAgentID != "" {
			assignedAgentID = order.AssignedAgentID
		}

		return c.JSON(fiber.Map{
			"id":            order.ID,
			"order_number":  order.OrderNumber,
			"status":        order.Status,
			"payment_type":  order.PaymentType,
			"order_value":   order.OrderValue,
			"created_at":    order.CreatedAt,
			"customer_name": order.CustomerName,
			"drop_address": fiber.Map{
				"line1":   order.DropLine1,
				"city":    order.DropCity,
				"pincode": order.DropPincode,
			},
			"charge_breakdown": fiber.Map{
				"base_charge":    order.BaseCharge,
				"cod_surcharge":  order.CODSurcharge,
				"fuel_surcharge": order.FuelSurcharge,
				"gst_amount":     order.GSTAmount,
				"total_charge":   order.TotalCharge,
			},
			"total_charge":     order.TotalCharge,
			"assigned_agent_id": assignedAgentID,
			"status_history":   history,
		})
	}
}

func AssignOrder(pool *pgxpool.Pool) fiber.Handler {
	type assignRequest struct {
		Mode    string `json:"mode"`
		AgentID string `json:"agent_id"`
	}
	return func(c fiber.Ctx) error {
		var req assignRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		switch req.Mode {
		case "AUTO":
			result, err := services.AssignAgent(c.Context(), pool, c.Params("id"))
			if err != nil {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(result)
		case "MANUAL":
			if req.AgentID == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "MANUAL mode needs agent_id"})
			}
			result, err := services.ManualAssign(c.Context(), pool, c.Params("id"), req.AgentID)
			if err != nil {
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
			}
			return c.JSON(result)
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "mode must be AUTO or MANUAL"})
		}
	}
}

func UpdateOrderStatus(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Status string `json:"status"`
			Notes  string `json:"notes"`
		}
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		// Actor identity comes from the verified token; the body's
		// actor_id / actor_type are never trusted.
		result, err := services.UpdateStatus(c.Context(), pool, c.Params("id"), &services.UpdateStatusInput{
			Status:  req.Status,
			Notes:   req.Notes,
			Role:    roleFrom(c),
			ActorID: userIDFrom(c),
		})
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

func roleFrom(c fiber.Ctx) string {
	role, _ := c.Locals("role").(string)
	return role
}
