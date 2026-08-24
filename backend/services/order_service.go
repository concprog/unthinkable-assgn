package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
)

// ---------------------------------------------------------
// Order creation — ARCHITECTURE.md §4, the eight steps in
// order: zone detection, volumetric weight, chargeable
// weight, rate-card lookup, base charge, COD surcharge,
// fuel surcharge, GST. Nothing is final until the row is
// committed with the snapshotted breakdown.
// ---------------------------------------------------------

type OrderAddressInput struct {
	Line1   string `json:"line1"`
	Line2   string `json:"line2"`
	City    string `json:"city"`
	State   string `json:"state"`
	Pincode string `json:"pincode"`
}

type PackageInput struct {
	LengthCM       float64 `json:"length_cm"`
	BreadthCM      float64 `json:"breadth_cm"`
	HeightCM       float64 `json:"height_cm"`
	ActualWeightKG float64 `json:"actual_weight_kg"`
}

type CreateOrderInput struct {
	PickupAddress OrderAddressInput `json:"pickup_address"`
	DropAddress   OrderAddressInput `json:"drop_address"`
	Package       PackageInput      `json:"package"`
	OrderType     string            `json:"order_type"`
	PaymentType   string            `json:"payment_type"`
	OrderValue    float64           `json:"order_value"`
}

type CreateOrderResult struct {
	OrderID             string          `json:"order_id"`
	OrderNumber         string          `json:"order_number"`
	Status              string          `json:"status"`
	ChargeableWeightKG  float64         `json:"chargeable_weight_kg"`
	VolumetricWeightKG  float64         `json:"volumetric_weight_kg"`
	PickupZoneID        int             `json:"pickup_zone_id"`
	DropZoneID          int             `json:"drop_zone_id"`
	ChargeBreakdown     ChargeBreakdown `json:"charge_breakdown"`
}

func CreateOrder(ctx context.Context, pool *pgxpool.Pool, customerID string, in *CreateOrderInput) (*CreateOrderResult, error) {
	if err := validateCreateOrder(in); err != nil {
		return nil, err
	}

	pickupZoneID, err := DetectZone(ctx, pool, in.PickupAddress.Pincode)
	if err != nil {
		return nil, fmt.Errorf("pickup %s", err)
	}
	dropZoneID, err := DetectZone(ctx, pool, in.DropAddress.Pincode)
	if err != nil {
		return nil, fmt.Errorf("drop %s", err)
	}

	rate, err := repository.FindActiveRateCardForLane(ctx, pool, in.OrderType, pickupZoneID, dropZoneID)
	if err != nil {
		return nil, fmt.Errorf("no active %s rate card for zone lane %d→%d", in.OrderType, pickupZoneID, dropZoneID)
	}

	volumetric := VolumetricWeightWithDivisor(
		in.Package.LengthCM, in.Package.BreadthCM, in.Package.HeightCM, float64(rate.VolumetricDivisor))
	chargeable := ChargeableWeight(in.Package.ActualWeightKG, volumetric)

	breakdown := ComputeCharge(&RateQuote{
		RateCardID:            rate.RateCardID,
		BasePrice:             rate.BasePrice,
		BaseWeightKG:          rate.BaseWeightKG,
		AdditionalPerKG:       rate.AdditionalPerKG,
		CODSurchargeFlat:      rate.CODSurchargeFlat,
		CODSurchargePct:       rate.CODSurchargePct,
		FuelSurchargePct:      rate.FuelSurchargePct,
		GSTPct:                rate.GSTPct,
		MinChargeableWeightKG: rate.MinChargeableWeightKG,
	}, chargeable, in.OrderValue, in.PaymentType == "COD")

	var orderValue *float64
	if in.OrderValue > 0 {
		orderValue = &in.OrderValue
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	pickupAddrID, err := repository.InsertAddress(ctx, tx, &repository.InsertAddressInput{
		UserID: customerID, Line1: in.PickupAddress.Line1, Line2: in.PickupAddress.Line2,
		City: in.PickupAddress.City, State: in.PickupAddress.State, Pincode: in.PickupAddress.Pincode,
	})
	if err != nil {
		return nil, fmt.Errorf("save pickup address: %w", err)
	}
	dropAddrID, err := repository.InsertAddress(ctx, tx, &repository.InsertAddressInput{
		UserID: customerID, Line1: in.DropAddress.Line1, Line2: in.DropAddress.Line2,
		City: in.DropAddress.City, State: in.DropAddress.State, Pincode: in.DropAddress.Pincode,
	})
	if err != nil {
		return nil, fmt.Errorf("save drop address: %w", err)
	}

	orderID, err := repository.CreateOrder(ctx, tx, &repository.InsertOrderInput{
		CustomerID:       customerID,
		CreatedByID:      customerID,
		PickupAddressID:  pickupAddrID,
		DropAddressID:    dropAddrID,
		PickupZoneID:     pickupZoneID,
		DropZoneID:       dropZoneID,
		OrderType:        in.OrderType,
		PaymentType:      in.PaymentType,
		LengthCM:         in.Package.LengthCM,
		BreadthCM:        in.Package.BreadthCM,
		HeightCM:         in.Package.HeightCM,
		ActualWeightKG:   in.Package.ActualWeightKG,
		VolumetricWeight: round2(volumetric),
		ChargeableWeight: round2(chargeable),
		RateCardID:       rate.RateCardID,
		BaseCharge:       breakdown.BaseCharge,
		CODSurcharge:     breakdown.CODSurcharge,
		FuelSurcharge:    breakdown.FuelSurcharge,
		GSTAmount:        breakdown.GSTAmount,
		TotalCharge:      breakdown.TotalCharge,
		OrderValue:       orderValue,
	})
	if err != nil {
		return nil, err
	}

	if err := repository.InsertStatusHistory(ctx, tx, orderID, "CREATED", "CUSTOMER", customerID, "order created"); err != nil {
		return nil, fmt.Errorf("write status history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &CreateOrderResult{
		OrderID:             orderID,
		Status:              "CREATED",
		ChargeableWeightKG:  round2(chargeable),
		VolumetricWeightKG:  round2(volumetric),
		PickupZoneID:        pickupZoneID,
		DropZoneID:          dropZoneID,
		ChargeBreakdown:     breakdown,
	}, nil
}

func validateCreateOrder(in *CreateOrderInput) error {
	for _, a := range []struct{ label string; addr OrderAddressInput }{
		{"pickup address", in.PickupAddress},
		{"drop address", in.DropAddress},
	} {
		if a.addr.Line1 == "" || a.addr.City == "" || a.addr.Pincode == "" {
			return fmt.Errorf("%s needs line1, city and pincode", a.label)
		}
	}
	p := in.Package
	if p.LengthCM <= 0 || p.BreadthCM <= 0 || p.HeightCM <= 0 || p.ActualWeightKG <= 0 {
		return fmt.Errorf("package dimensions and weight must be positive")
	}
	if in.OrderType != "B2B" && in.OrderType != "B2C" {
		return fmt.Errorf("order_type must be B2B or B2C")
	}
	if in.PaymentType != "PREPAID" && in.PaymentType != "COD" {
		return fmt.Errorf("payment_type must be PREPAID or COD")
	}
	if in.PaymentType == "COD" && in.OrderValue <= 0 {
		return fmt.Errorf("COD orders need a positive order_value")
	}
	return nil
}
