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

// ---------------------------------------------------------
// Zone area management — a pincode belongs to exactly one
// zone (UNIQUE), so adding an existing pincode to another
// zone reassigns it.
// ---------------------------------------------------------

type AreaRow struct {
	Pincode string  `json:"pincode"`
	City    *string `json:"city,omitempty"`
	State   *string `json:"state,omitempty"`
}

func ListZoneAreas(ctx context.Context, pool *pgxpool.Pool, zoneID int) ([]AreaRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT pincode, city, state FROM zone_areas WHERE zone_id = $1 ORDER BY pincode`, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var areas []AreaRow
	for rows.Next() {
		var a AreaRow
		if err := rows.Scan(&a.Pincode, &a.City, &a.State); err != nil {
			return nil, err
		}
		areas = append(areas, a)
	}
	return areas, rows.Err()
}

func ZoneExists(ctx context.Context, pool *pgxpool.Pool, zoneID int) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM zones WHERE id = $1)`, zoneID).Scan(&exists)
	return exists, err
}

// AddZoneAreas upserts each pincode into the zone; a pincode already
// mapped elsewhere is moved. Returns how many areas the zone has after.
func AddZoneAreas(ctx context.Context, pool *pgxpool.Pool, zoneID int, pincodes []PincodeInput) ([]AreaRow, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	for _, zc := range pincodes {
		_, err = tx.Exec(ctx,
			`INSERT INTO zone_areas (zone_id, pincode, city, state)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (pincode) DO UPDATE
			 SET zone_id = EXCLUDED.zone_id, city = EXCLUDED.city, state = EXCLUDED.state`,
			zoneID, zc.Pincode, zc.City, zc.State)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return ListZoneAreas(ctx, pool, zoneID)
}

func RemoveZoneArea(ctx context.Context, pool *pgxpool.Pool, zoneID int, pincode string) error {
	tag, err := pool.Exec(ctx,
		`DELETE FROM zone_areas WHERE zone_id = $1 AND pincode = $2`, zoneID, pincode)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
