package handlers

import (
	"net/url"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
)

type CreateZoneRequest struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
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

// ListZones — GET /api/admin/zones: every zone plus how many
// pincodes are mapped into it.
func ListZones(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		zones, err := repository.ListZones(c.Context(), pool)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list zones"})
		}
		if zones == nil {
			zones = []repository.ZoneRow{}
		}
		return c.JSON(zones)
	}
}

// ListRateCards — GET /api/admin/rate-cards: all cards with their
// zone-pair lanes; the UI picks the active card per order type.
func ListRateCards(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		cards, err := repository.ListRateCards(c.Context(), pool)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list rate cards"})
		}
		if cards == nil {
			cards = []repository.RateCardRow{}
		}
		return c.JSON(cards)
	}
}

// EditLane — PATCH /api/admin/rate-cards/:id/lanes: upsert one
// zone-pair price row. Adding a new lane needs no schema change.
func EditLane(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			FromZoneID           int     `json:"from_zone_id"`
			ToZoneID             int     `json:"to_zone_id"`
			BasePrice            float64 `json:"base_price"`
			AdditionalPricePerKG float64 `json:"additional_price_per_kg"`
		}
		if err := c.Bind().Body(&req); err != nil || req.FromZoneID <= 0 || req.ToZoneID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "from_zone_id and to_zone_id are required"})
		}

		rateCardID, err := strconv.Atoi(c.Params("id"))
		if err != nil || rateCardID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid rate card id"})
		}

		lane, err := repository.UpsertLane(c.Context(), pool, &repository.UpsertLaneInput{
			RateCardID:           rateCardID,
			FromZoneID:           req.FromZoneID,
			ToZoneID:             req.ToZoneID,
			BasePrice:            req.BasePrice,
			AdditionalPricePerKG: req.AdditionalPricePerKG,
		})
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save lane"})
		}
		return c.JSON(lane)
	}
}

// ListAdminOrders — GET /api/admin/orders?zone=&status=&status=&agent=
// `status` may repeat (the assignments page asks for CREATED and FAILED
// in one call), so the raw query string is parsed directly.
func ListAdminOrders(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		filter := repository.AdminOrderFilter{Zone: c.Query("zone"), AgentID: c.Query("agent")}

		if qs := string(c.Request().URI().QueryString()); qs != "" {
			if values, err := url.ParseQuery(qs); err == nil {
				filter.Statuses = values["status"]
			}
		}

		orders, err := repository.ListAdminOrders(c.Context(), pool, filter)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list orders"})
		}
		if orders == nil {
			orders = []repository.AdminOrderRow{}
		}
		return c.JSON(orders)
	}
}

// NearbyAgents — GET /api/admin/orders/:id/nearby-agents: ranked by
// the same Haversine distance the auto engine uses, busy agents included.
func NearbyAgents(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		orderID := c.Params("id")

		zoneID, lat, lng, err := repository.GetOrderPickupInfo(c.Context(), pool, orderID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
		}

		agents, err := repository.ListNearbyAgents(c.Context(), pool, zoneID, lat, lng)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list agents"})
		}
		if agents == nil {
			agents = []repository.NearbyAgentRow{}
		}
		return c.JSON(agents)
	}
}
