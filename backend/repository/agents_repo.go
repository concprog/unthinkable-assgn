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
