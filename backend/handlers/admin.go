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

// ListZoneAreas — GET /api/admin/zones/:id/areas: every pincode
// mapped into the zone.
func ListZoneAreas(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		zoneID, err := strconv.Atoi(c.Params("id"))
		if err != nil || zoneID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid zone id"})
		}
		if ok, err := repository.ZoneExists(c.Context(), pool, zoneID); err != nil || !ok {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "zone not found"})
		}

		areas, err := repository.ListZoneAreas(c.Context(), pool, zoneID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list areas"})
		}
		if areas == nil {
			areas = []repository.AreaRow{}
		}
		return c.JSON(areas)
	}
}

// AddZoneAreas — POST /api/admin/zones/:id/areas {pincodes: [{pincode, city, state}]}.
// A pincode already assigned to another zone is moved here.
func AddZoneAreas(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Pincodes []repository.PincodeInput `json:"pincodes"`
		}
		if err := c.Bind().Body(&req); err != nil || len(req.Pincodes) == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "pincodes array is required"})
		}
		for _, p := range req.Pincodes {
			if len(p.Pincode) < 3 || len(p.Pincode) > 10 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "each pincode must be 3-10 characters"})
			}
		}

		zoneID, err := strconv.Atoi(c.Params("id"))
		if err != nil || zoneID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid zone id"})
		}
		if ok, err := repository.ZoneExists(c.Context(), pool, zoneID); err != nil || !ok {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "zone not found"})
		}

		areas, err := repository.AddZoneAreas(c.Context(), pool, zoneID, req.Pincodes)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to add areas"})
		}
		return c.JSON(areas)
	}
}

// RemoveZoneArea — DELETE /api/admin/zones/:id/areas/:pincode.
func RemoveZoneArea(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		zoneID, err := strconv.Atoi(c.Params("id"))
		if err != nil || zoneID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid zone id"})
		}
		pincode := c.Params("pincode")
		if pincode == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid pincode"})
		}

		if err := repository.RemoveZoneArea(c.Context(), pool, zoneID, pincode); err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "pincode not found in this zone"})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// CreateRateCard — POST /api/admin/rate-cards: a new card for B2B or
// B2C. Created inactive; use PATCH is_active to go live (retires the
// previous active card of that type).
func CreateRateCard(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req repository.CreateRateCardInput
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		if req.OrderType != "B2B" && req.OrderType != "B2C" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "order_type must be B2B or B2C"})
		}
		if req.Name == "" {
			req.Name = "Default card"
		}

		id, err := repository.CreateRateCard(c.Context(), pool, &req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create rate card"})
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
	}
}

// UpdateRateCard — PATCH /api/admin/rate-cards/:id: surcharges,
// volumetric divisor, name and/or is_active toggle.
func UpdateRateCard(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := strconv.Atoi(c.Params("id"))
		if err != nil || id <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid rate card id"})
		}

		var req repository.UpdateRateCardInput
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		if err := repository.UpdateRateCard(c.Context(), pool, id, &req); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update rate card"})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// DeleteRateCard — DELETE /api/admin/rate-cards/:id: only inactive
// cards; orders reference active ones.
func DeleteRateCard(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		id, err := strconv.Atoi(c.Params("id"))
		if err != nil || id <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid rate card id"})
		}
		if err := repository.DeleteRateCard(c.Context(), pool, id); err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
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
