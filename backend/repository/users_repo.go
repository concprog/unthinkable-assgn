package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
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

// ---------------------------------------------------------
// First-party auth — the schema's password_hash column with
// app-level RBAC via the role enum.
// ---------------------------------------------------------

type CreateUserInput struct {
	FullName     string
	Email        string
	Phone        string
	PasswordHash string
	Role         string // customer | agent | admin
}

func CreateUser(ctx context.Context, pool *pgxpool.Pool, in *CreateUserInput) (string, error) {
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO users (role, full_name, email, phone, password_hash)
		 VALUES ($1::user_role, $2, $3, $4, $5) RETURNING id`,
		in.Role, in.FullName, in.Email, in.Phone, in.PasswordHash).Scan(&id)
	if isUniqueViolation(err) {
		var field string
		_ = pool.QueryRow(ctx,
			`SELECT 'email' WHERE EXISTS (SELECT 1 FROM users WHERE email = $1)
			 UNION ALL SELECT 'phone' WHERE EXISTS (SELECT 1 FROM users WHERE phone = $2) LIMIT 1`,
			in.Email, in.Phone).Scan(&field)
		if field == "" {
			field = "email or phone"
		}
		return "", fmt.Errorf("an account with that %s already exists", field)
	}
	return id, err
}

type UserAuthRow struct {
	ID            string
	Role          string
	FullName      string
	PasswordHash  *string
	IsActive      bool
	EmailVerified bool
}

func GetUserByEmail(ctx context.Context, pool *pgxpool.Pool, email string) (*UserAuthRow, error) {
	row := &UserAuthRow{}
	err := pool.QueryRow(ctx,
		`SELECT id, role::text, full_name, password_hash, is_active, email_verified
		 FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&row.ID, &row.Role, &row.FullName, &row.PasswordHash, &row.IsActive, &row.EmailVerified)
	if err != nil {
		return nil, pgx.ErrNoRows
	}
	return row, nil
}

type UserProfileRow struct {
	ID            string `json:"id"`
	Role          string `json:"role"`
	FullName      string `json:"full_name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

func GetUserByID(ctx context.Context, pool *pgxpool.Pool, id string) (*UserProfileRow, error) {
	row := &UserProfileRow{}
	err := pool.QueryRow(ctx,
		`SELECT id, role::text, full_name, email, email_verified FROM users WHERE id = $1 AND is_active = true`, id).
		Scan(&row.ID, &row.Role, &row.FullName, &row.Email, &row.EmailVerified)
	if err != nil {
		return nil, pgx.ErrNoRows
	}
	return row, nil
}

func SetEmailVerified(ctx context.Context, pool *pgxpool.Pool, id string) error {
	_, err := pool.Exec(ctx,
		`UPDATE users SET email_verified = true WHERE id = $1`, id)
	return err
}

// GetUserEmail resolves just the recipient address (verification mail).
func GetUserEmail(ctx context.Context, pool *pgxpool.Pool, id string) (string, string, error) {
	var email, name string
	err := pool.QueryRow(ctx,
		`SELECT email, full_name FROM users WHERE id = $1 AND is_active = true`, id).
		Scan(&email, &name)
	return email, name, err
}
