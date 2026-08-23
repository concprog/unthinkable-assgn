package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
)

type AgentCandidate struct {
	ID         string
	Name       string
	DistanceKm float64
}

func AssignAgent(ctx context.Context, pool *pgxpool.Pool, orderID string) (interface{}, error) {
	pickupZoneID, pickupLat, pickupLng, err := repository.GetOrderPickupInfo(ctx, pool, orderID)
	if err != nil {
		return nil, fmt.Errorf("order %s not found", orderID)
	}

	candidates, err := repository.ListAvailableAgents(ctx, pool, pickupZoneID)
	if err != nil || len(candidates) == 0 {
		return nil, fmt.Errorf("no available agent in pickup zone")
	}

	best := candidates[0]
	bestDist := HaversineKm(pickupLat, pickupLng, best.Lat, best.Lng)
	for _, a := range candidates[1:] {
		d := HaversineKm(pickupLat, pickupLng, a.Lat, a.Lng)
		if d < bestDist {
			best = a
			bestDist = d
		}
	}

	if err := repository.AssignAgentToOrder(ctx, pool, orderID, best.UserID); err != nil {
		return nil, err
	}

	if err := repository.InsertStatusHistory(ctx, pool, orderID, "ASSIGNED", "SYSTEM", "", "auto-assigned"); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"order_id": orderID,
		"status":   "ASSIGNED",
		"assigned_agent": map[string]interface{}{
			"id":          best.UserID,
			"name":        best.Name,
			"distance_km": round2(bestDist),
		},
	}, nil
}

func UpdateStatus(ctx context.Context, pool *pgxpool.Pool, orderID string, req interface{}) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func Reschedule(ctx context.Context, pool *pgxpool.Pool, orderID, requestedDate string) (interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}
