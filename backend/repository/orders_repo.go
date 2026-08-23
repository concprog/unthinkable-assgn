package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentCandidateRow struct {
	UserID string
	Name   string
	Lat    float64
	Lng    float64
}

func GetOrderPickupInfo(ctx context.Context, pool *pgxpool.Pool, orderID string) (int, float64, float64, error) {
	var zoneID int
	var lat, lng float64
	err := pool.QueryRow(ctx,
		`SELECT o.pickup_zone_id, a.latitude, a.longitude
		 FROM orders o JOIN addresses a ON a.id = o.pickup_address_id
		 WHERE o.id = $1`, orderID).Scan(&zoneID, &lat, &lng)
	if err != nil {
		return 0, 0, 0, pgx.ErrNoRows
	}
	return zoneID, lat, lng, nil
}

func ListAvailableAgents(ctx context.Context, pool *pgxpool.Pool, zoneID int) ([]AgentCandidateRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT ap.user_id, u.full_name, ap.current_latitude, ap.current_longitude
		 FROM agent_profiles ap JOIN users u ON u.id = ap.user_id
		 WHERE ap.zone_id = $1 AND ap.availability = 'AVAILABLE' AND u.is_active = true`,
		zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []AgentCandidateRow
	for rows.Next() {
		var a AgentCandidateRow
		if err := rows.Scan(&a.UserID, &a.Name, &a.Lat, &a.Lng); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func AssignAgentToOrder(ctx context.Context, pool *pgxpool.Pool, orderID, agentID string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE orders SET assigned_agent_id = $1, status = 'ASSIGNED', updated_at = now() WHERE id = $2`,
		agentID, orderID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE agent_profiles SET availability = 'BUSY', updated_at = now() WHERE user_id = $1`,
		agentID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func InsertStatusHistory(ctx context.Context, pool *pgxpool.Pool, orderID, status, actorType, actorID, notes string) error {
	var actorArg interface{}
	if actorID != "" {
		actorArg = actorID
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO order_status_history (order_id, status, actor_type, actor_id, notes)
		 VALUES ($1, $2, $3, $4, $5)`,
		orderID, status, actorType, actorArg, notes)
	return err
}

type OrderRow struct {
	ID          string
	OrderNumber string
	Status      string
	CreatedAt   time.Time
}

func GetOrderByID(ctx context.Context, pool *pgxpool.Pool, id string) (*OrderRow, error) {
	row := &OrderRow{}
	err := pool.QueryRow(ctx,
		`SELECT id, order_number, status, created_at FROM orders WHERE id = $1`, id).
		Scan(&row.ID, &row.OrderNumber, &row.Status, &row.CreatedAt)
	if err != nil {
		return nil, pgx.ErrNoRows
	}
	return row, nil
}
