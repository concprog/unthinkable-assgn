package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PincodeInput struct {
	Pincode string `json:"pincode"`
	City    string `json:"city"`
	State   string `json:"state"`
}

func FindZoneByPincode(ctx context.Context, pool *pgxpool.Pool, pincode string) (int, error) {
	var zoneID int
	err := pool.QueryRow(ctx,
		`SELECT zone_id FROM zone_areas WHERE pincode = $1`, pincode).Scan(&zoneID)
	if err != nil {
		return 0, pgx.ErrNoRows
	}
	return zoneID, nil
}

func CreateZone(ctx context.Context, pool *pgxpool.Pool, name, description *string, pincodes []PincodeInput) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var zoneID int
	err = tx.QueryRow(ctx,
		`INSERT INTO zones (name, description) VALUES ($1, $2) RETURNING id`,
		name, description).Scan(&zoneID)
	if err != nil {
		return 0, err
	}

	for _, zc := range pincodes {
		_, err = tx.Exec(ctx,
			`INSERT INTO zone_areas (zone_id, pincode, city, state) VALUES ($1, $2, $3, $4)`,
			zoneID, zc.Pincode, zc.City, zc.State)
		if err != nil {
			return 0, err
		}
	}

	return zoneID, tx.Commit(ctx)
}
