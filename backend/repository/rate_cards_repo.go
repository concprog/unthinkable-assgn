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
	ID       int       `json:"id"`
	OrderType string   `json:"order_type"`
	IsActive bool      `json:"is_active"`
	Lanes    []LaneRow `json:"lanes"`
}

func ListRateCards(ctx context.Context, pool *pgxpool.Pool) ([]RateCardRow, error) {
	cardRows, err := pool.Query(ctx,
		`SELECT id, order_type::text, is_active FROM rate_cards ORDER BY is_active DESC, order_type, id`)
	if err != nil {
		return nil, err
	}
	defer cardRows.Close()

	var cards []RateCardRow
	for cardRows.Next() {
		var c RateCardRow
		if err := cardRows.Scan(&c.ID, &c.OrderType, &c.IsActive); err != nil {
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
