package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type RateCardZoneRate struct {
	RateCardID            int
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
		`SELECT rc.id, zr.base_price, zr.base_weight_kg, zr.additional_price_per_kg, zr.min_chargeable_weight_kg,
		        rc.cod_surcharge_flat, rc.cod_surcharge_pct, rc.fuel_surcharge_pct, rc.gst_pct
		 FROM rate_cards rc
		 JOIN rate_card_zone_rates zr ON zr.rate_card_id = rc.id
		 WHERE rc.order_type = $1 AND rc.is_active = true
		   AND zr.from_zone_id = $2 AND zr.to_zone_id = $3`,
		orderType, fromZoneID, toZoneID).
		Scan(&rate.RateCardID, &rate.BasePrice, &rate.BaseWeightKG, &rate.AdditionalPerKG, &rate.MinChargeableWeightKG,
			&rate.CODSurchargeFlat, &rate.CODSurchargePct, &rate.FuelSurchargePct, &rate.GSTPct)
	if err != nil {
		return nil, err
	}

	return rate, nil
}
