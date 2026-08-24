package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentProfileRow struct {
	UserID string
	Name   string
	Lat    float64
	Lng    float64
}

func UpdateAgentLocation(ctx context.Context, pool *pgxpool.Pool, agentID string, lat, lng float64) error {
	tag, err := pool.Exec(ctx,
		`UPDATE agent_profiles
		 SET current_latitude = $2, current_longitude = $3, last_location_at = now(), updated_at = now()
		 WHERE user_id = $1`,
		agentID, lat, lng)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func GetAgentProfile(ctx context.Context, pool *pgxpool.Pool, agentID string) (*AgentProfileRow, error) {
	row := &AgentProfileRow{}
	err := pool.QueryRow(ctx,
		`SELECT ap.user_id, u.full_name, ap.current_latitude, ap.current_longitude
		 FROM agent_profiles ap JOIN users u ON u.id = ap.user_id
		 WHERE ap.user_id = $1`, agentID).
		Scan(&row.UserID, &row.Name, &row.Lat, &row.Lng)
	if err != nil {
		return nil, errors.New("agent profile not found")
	}
	return row, nil
}

// ---------------------------------------------------------
// /agents/me/* — the agent app reads its own data from the
// token subject, never from a path param.
// ---------------------------------------------------------

type AgentTaskRow struct {
	ID          string `json:"id"`
	OrderNumber string `json:"order_number"`
	Status      string `json:"status"`
	PickupZoneID int   `json:"pickup_zone_id"`
}

func ListAssignedOrders(ctx context.Context, pool *pgxpool.Pool, agentID string) ([]AgentTaskRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, order_number, status::text, pickup_zone_id
		 FROM orders WHERE assigned_agent_id = $1
		 ORDER BY created_at DESC LIMIT 100`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []AgentTaskRow
	for rows.Next() {
		var t AgentTaskRow
		if err := rows.Scan(&t.ID, &t.OrderNumber, &t.Status, &t.PickupZoneID); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

type AvailabilityRow struct {
	UserID       string  `json:"user_id"`
	Name         string  `json:"name"`
	VehicleType  *string `json:"vehicle_type"`
	ZoneID       *int    `json:"zone_id"`
	Availability string  `json:"availability"`
}

func UpdateAvailability(ctx context.Context, pool *pgxpool.Pool, agentID, availability string) (*AvailabilityRow, error) {
	row := &AvailabilityRow{UserID: agentID}
	err := pool.QueryRow(ctx,
		`UPDATE agent_profiles ap
		 SET availability = $2::agent_availability, updated_at = now()
		 FROM users u
		 WHERE u.id = ap.user_id AND ap.user_id = $1
		 RETURNING u.full_name, ap.vehicle_type, ap.zone_id, ap.availability::text`,
		agentID, availability).
		Scan(&row.Name, &row.VehicleType, &row.ZoneID, &row.Availability)
	if err != nil {
		return nil, errors.New("agent profile not found")
	}
	return row, nil
}

// ---------------------------------------------------------
// Manual-assignment candidates — every agent in the pickup
// zone (busy ones included: the admin sees why), ranked by
// the same Haversine distance the auto engine uses.
// ---------------------------------------------------------

type NearbyAgentRow struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	DistanceKm   float64 `json:"distance_km"`
	Availability string  `json:"availability"`
}

func ListNearbyAgents(ctx context.Context, pool *pgxpool.Pool, zoneID int, lat, lng float64) ([]NearbyAgentRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT ap.user_id, u.full_name,
		        6371 * 2 * atan2(
		          sqrt(
		            sin(radians(ap.current_latitude - $2) / 2) * sin(radians(ap.current_latitude - $2) / 2) +
		            cos(radians($2)) * cos(radians(ap.current_latitude)) *
		            sin(radians(ap.current_longitude - $3) / 2) * sin(radians(ap.current_longitude - $3) / 2)
		          ),
		          sqrt(1 - pow(
		            sin(radians(ap.current_latitude - $2) / 2) * sin(radians(ap.current_latitude - $2) / 2) +
		            cos(radians($2)) * cos(radians(ap.current_latitude)) *
		            sin(radians(ap.current_longitude - $3) / 2) * sin(radians(ap.current_longitude - $3) / 2)
		          , 2))
		        ) AS distance_km,
		        ap.availability::text
		 FROM agent_profiles ap JOIN users u ON u.id = ap.user_id
		 WHERE ap.zone_id = $1 AND u.is_active = true
		   AND ap.current_latitude IS NOT NULL AND ap.current_longitude IS NOT NULL
		 ORDER BY distance_km ASC LIMIT 20`,
		zoneID, lat, lng)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []NearbyAgentRow
	for rows.Next() {
		var a NearbyAgentRow
		if err := rows.Scan(&a.ID, &a.Name, &a.DistanceKm, &a.Availability); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}
