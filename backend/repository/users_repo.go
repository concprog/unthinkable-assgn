package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ClerkUserData struct {
	ID    string `json:"id"`
	Name  string `json:"full_name"`
	Email string `json:"email"`
}

func UpsertUserFromClerk(ctx context.Context, pool *pgxpool.Pool, data json.RawMessage) error {
	var u ClerkUserData
	if err := json.Unmarshal(data, &u); err != nil {
		return err
	}

	_, err := pool.Exec(ctx,
		`INSERT INTO users (role, full_name, email, external_auth_id, is_active)
		 VALUES ('customer', $1, $2, $3, true)
		 ON CONFLICT (external_auth_id) DO UPDATE
		 SET full_name = EXCLUDED.full_name,
		     email = EXCLUDED.email,
		     updated_at = now()`,
		u.Name, u.Email, u.ID)

	return err
}

func DeactivateUserFromClerk(ctx context.Context, pool *pgxpool.Pool, data json.RawMessage) error {
	var payload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	_, err := pool.Exec(ctx,
		`UPDATE users SET is_active = false, updated_at = now() WHERE external_auth_id = $1`,
		payload.Data.ID)

	return err
}
