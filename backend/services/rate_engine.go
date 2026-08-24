package services

import (
	"math"
)

const (
	DefaultVolumetricDivisor = 5000.0
)

// VolumetricWeightWithDivisor lets an active rate card override the
// industry-standard divisor of 5000 (rate_cards.volumetric_divisor).
func VolumetricWeightWithDivisor(l, b, h, divisor float64) float64 {
	if divisor <= 0 {
		divisor = DefaultVolumetricDivisor
	}
	return (l * b * h) / divisor
}

func VolumetricWeight(l, b, h float64) float64 {
	return VolumetricWeightWithDivisor(l, b, h, DefaultVolumetricDivisor)
}

func ChargeableWeight(actual, volumetric float64) float64 {
	return math.Max(actual, volumetric)
}

type RateQuote struct {
	RateCardID            int
	BasePrice             float64
	BaseWeightKG          float64
	AdditionalPerKG       float64
	CODSurchargeFlat      float64
	CODSurchargePct       float64
	FuelSurchargePct      float64
	GSTPct                float64
	MinChargeableWeightKG float64
}

type ChargeBreakdown struct {
	BaseCharge    float64 `json:"base_charge"`
	CODSurcharge  float64 `json:"cod_surcharge"`
	FuelSurcharge float64 `json:"fuel_surcharge"`
	GSTAmount     float64 `json:"gst_amount"`
	TotalCharge   float64 `json:"total_charge"`
}

func ComputeCharge(q *RateQuote, chargeableWeight, orderValue float64, isCOD bool) ChargeBreakdown {
	billable := math.Max(chargeableWeight, q.MinChargeableWeightKG)
	extraKg := billable - q.BaseWeightKG
	if extraKg < 0 {
		extraKg = 0
	}

	base := q.BasePrice + round2(extraKg*q.AdditionalPerKG)

	cod := 0.0
	if isCOD {
		cod = round2(q.CODSurchargeFlat + (orderValue*q.CODSurchargePct)/100)
	}

	fuel := round2((base * q.FuelSurchargePct) / 100)
	gst := round2(((base + cod + fuel) * q.GSTPct) / 100)
	total := round2(base + cod + fuel + gst)

	return ChargeBreakdown{
		BaseCharge:    base,
		CODSurcharge:  cod,
		FuelSurcharge: fuel,
		GSTAmount:     gst,
		TotalCharge:   total,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
