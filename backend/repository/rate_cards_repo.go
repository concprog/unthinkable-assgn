package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RateCardZoneRate struct {
	RateCardID            int
	VolumetricDivisor     int
	BasePrice             float64
	BaseWeightKG          float64
	AdditionalPerKG       float64
	MinChargeableWeightKG float64
	CODSurchargeFlat      float64
	CODSurchargePct       float64
	FuelSurchargePct      float64
	GSTPct                float64
}

func FindActiveRateCardForLane(ctx context.Context, pool *pgxpool.Pool, orderType string, fromZoneID, toZoneID int) (*RateCardZoneRate, error) {
	rate := &RateCardZoneRate{}
	err := pool.QueryRow(ctx,
		`SELECT rc.id, rc.volumetric_divisor, zr.base_price, zr.base_weight_kg, zr.additional_price_per_kg, zr.min_chargeable_weight_kg,
		        rc.cod_surcharge_flat, rc.cod_surcharge_pct, rc.fuel_surcharge_pct, rc.gst_pct
		 FROM rate_cards rc
		 JOIN rate_card_zone_rates zr ON zr.rate_card_id = rc.id
		 WHERE rc.order_type = $1 AND rc.is_active = true
		   AND zr.from_zone_id = $2 AND zr.to_zone_id = $3`,
		orderType, fromZoneID, toZoneID).
		Scan(&rate.RateCardID, &rate.VolumetricDivisor, &rate.BasePrice, &rate.BaseWeightKG, &rate.AdditionalPerKG, &rate.MinChargeableWeightKG,
			&rate.CODSurchargeFlat, &rate.CODSurchargePct, &rate.FuelSurchargePct, &rate.GSTPct)
	if err != nil {
		return nil, err
	}

	return rate, nil
}

// ---------------------------------------------------------
// Admin views — zones with area counts, rate cards with all
// their zone-pair lanes, and lane price edits.
// ---------------------------------------------------------

type ZoneRow struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	AreaCount   int     `json:"area_count"`
}

func ListZones(ctx context.Context, pool *pgxpool.Pool) ([]ZoneRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT z.id, z.name, z.description, count(za.id)
		 FROM zones z LEFT JOIN zone_areas za ON za.zone_id = z.id
		 GROUP BY z.id ORDER BY z.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []ZoneRow
	for rows.Next() {
		var z ZoneRow
		if err := rows.Scan(&z.ID, &z.Name, &z.Description, &z.AreaCount); err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

type LaneRow struct {
	FromZoneID           int     `json:"from_zone_id"`
	ToZoneID             int     `json:"to_zone_id"`
	FromZoneName         *string `json:"from_zone_name,omitempty"`
	ToZoneName           *string `json:"to_zone_name,omitempty"`
	BasePrice            float64 `json:"base_price"`
	AdditionalPricePerKG float64 `json:"additional_price_per_kg"`
}

type RateCardRow struct {
	ID                int       `json:"id"`
	OrderType         string    `json:"order_type"`
	Name              *string   `json:"name,omitempty"`
	VolumetricDivisor int       `json:"volumetric_divisor"`
	CODSurchargeFlat  float64   `json:"cod_surcharge_flat"`
	CODSurchargePct   float64   `json:"cod_surcharge_pct"`
	FuelSurchargePct  float64   `json:"fuel_surcharge_pct"`
	GSTPct            float64   `json:"gst_pct"`
	IsActive          bool      `json:"is_active"`
	Lanes             []LaneRow `json:"lanes"`
}

func ListRateCards(ctx context.Context, pool *pgxpool.Pool) ([]RateCardRow, error) {
	cardRows, err := pool.Query(ctx,
		`SELECT id, order_type::text, name, volumetric_divisor,
		        cod_surcharge_flat, cod_surcharge_pct, fuel_surcharge_pct, gst_pct, is_active
		 FROM rate_cards ORDER BY is_active DESC, order_type, id`)
	if err != nil {
		return nil, err
	}
	defer cardRows.Close()

	var cards []RateCardRow
	for cardRows.Next() {
		var c RateCardRow
		if err := cardRows.Scan(&c.ID, &c.OrderType, &c.Name, &c.VolumetricDivisor,
			&c.CODSurchargeFlat, &c.CODSurchargePct, &c.FuelSurchargePct, &c.GSTPct, &c.IsActive); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	if err := cardRows.Err(); err != nil {
		return nil, err
	}

	lanes, err := listAllLanes(ctx, pool)
	if err != nil {
		return nil, err
	}
	for i := range cards {
		cards[i].Lanes = lanes[cards[i].ID]
		if cards[i].Lanes == nil {
			cards[i].Lanes = []LaneRow{}
		}
	}
	return cards, nil
}

func listAllLanes(ctx context.Context, pool *pgxpool.Pool) (map[int][]LaneRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT zr.rate_card_id, zr.from_zone_id, zr.to_zone_id,
		        zf.name, zt.name, zr.base_price, zr.additional_price_per_kg
		 FROM rate_card_zone_rates zr
		 JOIN zones zf ON zf.id = zr.from_zone_id
		 JOIN zones zt ON zt.id = zr.to_zone_id
		 ORDER BY zr.from_zone_id, zr.to_zone_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lanes := make(map[int][]LaneRow)
	for rows.Next() {
		var rateCardID int
		var lane LaneRow
		if err := rows.Scan(&rateCardID, &lane.FromZoneID, &lane.ToZoneID,
			&lane.FromZoneName, &lane.ToZoneName, &lane.BasePrice, &lane.AdditionalPricePerKG); err != nil {
			return nil, err
		}
		lanes[rateCardID] = append(lanes[rateCardID], lane)
	}
	return lanes, rows.Err()
}

type UpsertLaneInput struct {
	RateCardID           int
	FromZoneID           int
	ToZoneID             int
	BasePrice            float64
	AdditionalPricePerKG float64
}

func UpsertLane(ctx context.Context, pool *pgxpool.Pool, in *UpsertLaneInput) (*LaneRow, error) {
	tag, err := pool.Exec(ctx,
		`INSERT INTO rate_card_zone_rates
			(rate_card_id, from_zone_id, to_zone_id, base_price, additional_price_per_kg)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (rate_card_id, from_zone_id, to_zone_id)
		 DO UPDATE SET base_price = EXCLUDED.base_price,
		               additional_price_per_kg = EXCLUDED.additional_price_per_kg`,
		in.RateCardID, in.FromZoneID, in.ToZoneID, in.BasePrice, in.AdditionalPricePerKG)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, errors.New("lane not saved")
	}
	return &LaneRow{
		FromZoneID:           in.FromZoneID,
		ToZoneID:             in.ToZoneID,
		BasePrice:            in.BasePrice,
		AdditionalPricePerKG: in.AdditionalPricePerKG,
	}, nil
}

// ---------------------------------------------------------
// Rate card lifecycle — create a card for an order type and
// edit its surcharges/divisor. Only one ACTIVE card per order
// type is allowed (partial unique index), so activating one
// card deactivates the previous active card of that type.
// ---------------------------------------------------------

type CreateRateCardInput struct {
	OrderType         string  `json:"order_type"` // B2B | B2C
	Name              string  `json:"name"`
	VolumetricDivisor int     `json:"volumetric_divisor"`
	CODSurchargeFlat  float64 `json:"cod_surcharge_flat"`
	CODSurchargePct   float64 `json:"cod_surcharge_pct"`
	FuelSurchargePct  float64 `json:"fuel_surcharge_pct"`
	GSTPct            float64 `json:"gst_pct"`
}

func CreateRateCard(ctx context.Context, pool *pgxpool.Pool, in *CreateRateCardInput) (int, error) {
	if in.VolumetricDivisor <= 0 {
		in.VolumetricDivisor = 5000
	}
	var id int
	err := pool.QueryRow(ctx,
		`INSERT INTO rate_cards (order_type, name, volumetric_divisor,
			cod_surcharge_flat, cod_surcharge_pct, fuel_surcharge_pct, gst_pct)
		 VALUES ($1::order_type, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		in.OrderType, in.Name, in.VolumetricDivisor,
		in.CODSurchargeFlat, in.CODSurchargePct, in.FuelSurchargePct, in.GSTPct).Scan(&id)
	return id, err
}

// UpdateRateCardInput uses pointers so PATCH only touches sent fields.
type UpdateRateCardInput struct {
	Name              *string  `json:"name"`
	VolumetricDivisor *int     `json:"volumetric_divisor"`
	CODSurchargeFlat  *float64 `json:"cod_surcharge_flat"`
	CODSurchargePct   *float64 `json:"cod_surcharge_pct"`
	FuelSurchargePct  *float64 `json:"fuel_surcharge_pct"`
	GSTPct            *float64 `json:"gst_pct"`
	IsActive          *bool    `json:"is_active"`
}

func UpdateRateCard(ctx context.Context, pool *pgxpool.Pool, id int, in *UpdateRateCardInput) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Activating a card must retire the current active card of the same
	// order type first — uq_one_active_ratecard_per_type would reject it.
	if in.IsActive != nil && *in.IsActive {
		tag, err := tx.Exec(ctx,
			`UPDATE rate_cards SET is_active = false, effective_to = now()
			 WHERE order_type = (SELECT order_type FROM rate_cards WHERE id = $1)
			   AND is_active = true AND id <> $1`, id)
		if err != nil {
			return err
		}
		_ = tag
		_, err = tx.Exec(ctx,
			`UPDATE rate_cards SET is_active = true, effective_to = NULL WHERE id = $1`, id)
		if err != nil {
			return err
		}
	} else if in.IsActive != nil && !*in.IsActive {
		_, err = tx.Exec(ctx,
			`UPDATE rate_cards SET is_active = false, effective_to = now() WHERE id = $1`, id)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx,
		`UPDATE rate_cards SET
			name               = COALESCE($2, name),
			volumetric_divisor = COALESCE($3, volumetric_divisor),
			cod_surcharge_flat = COALESCE($4, cod_surcharge_flat),
			cod_surcharge_pct  = COALESCE($5, cod_surcharge_pct),
			fuel_surcharge_pct = COALESCE($6, fuel_surcharge_pct),
			gst_pct            = COALESCE($7, gst_pct)
		 WHERE id = $1`,
		id, in.Name, in.VolumetricDivisor, in.CODSurchargeFlat,
		in.CODSurchargePct, in.FuelSurchargePct, in.GSTPct)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteRateCard removes an inactive card; FK on orders.rate_card_id
// blocks deleting cards already used by orders.
func DeleteRateCard(ctx context.Context, pool *pgxpool.Pool, id int) error {
	tag, err := pool.Exec(ctx, `DELETE FROM rate_cards WHERE id = $1 AND is_active = false`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("rate card not found or still active/referenced by orders")
	}
	return nil
}
