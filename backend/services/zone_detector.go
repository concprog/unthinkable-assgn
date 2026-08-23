package services

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
)

func DetectZone(ctx context.Context, pool *pgxpool.Pool, pincode string) (int, error) {
	zoneID, err := repository.FindZoneByPincode(ctx, pool, pincode)
	if err != nil {
		return 0, fmt.Errorf("pincode %s not mapped to any zone", pincode)
	}
	return zoneID, nil
}

func HaversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	dLat := radians(lat2 - lat1)
	dLon := radians(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(radians(lat1))*math.Cos(radians(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func radians(deg float64) float64 {
	return deg * math.Pi / 180
}
