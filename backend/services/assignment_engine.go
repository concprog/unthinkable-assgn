package services

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
)

type AgentCandidate struct {
	ID         string
	Name       string
	DistanceKm float64
}

// ---------------------------------------------------------
// Auto-assignment — ARCHITECTURE.md §5: available agents in
// the pickup zone, Haversine-ranked, closest wins.
// ---------------------------------------------------------

func AssignAgent(ctx context.Context, pool *pgxpool.Pool, orderID string) (interface{}, error) {
	pickupZoneID, pickupLat, pickupLng, err := repository.GetOrderPickupInfo(ctx, pool, orderID)
	if err != nil {
		return nil, fmt.Errorf("order %s not found", orderID)
	}

	best, bestDist, err := pickClosestAgent(ctx, pool, pickupZoneID, pickupLat, pickupLng)
	if err != nil {
		return nil, err
	}

	if err := applyAssignment(ctx, pool, orderID, best.UserID); err != nil {
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

func pickClosestAgent(ctx context.Context, pool *pgxpool.Pool, zoneID int, lat, lng float64) (*repository.AgentCandidateRow, float64, error) {
	candidates, err := repository.ListAvailableAgents(ctx, pool, zoneID)
	if err != nil {
		return nil, 0, fmt.Errorf("no available agent in zone %d", zoneID)
	}
	if len(candidates) == 0 {
		return nil, 0, fmt.Errorf("no available agent in pickup zone")
	}

	best := candidates[0]
	bestDist := HaversineKm(lat, lng, best.Lat, best.Lng)
	for _, a := range candidates[1:] {
		d := HaversineKm(lat, lng, a.Lat, a.Lng)
		if d < bestDist {
			best = a
			bestDist = d
		}
	}
	return &best, bestDist, nil
}

func applyAssignment(ctx context.Context, pool *pgxpool.Pool, orderID, agentID string) error {
	if err := repository.AssignAgentToOrder(ctx, pool, orderID, agentID); err != nil {
		return err
	}
	return repository.InsertStatusHistory(ctx, pool, orderID, "ASSIGNED", "SYSTEM", "", "assigned to agent "+agentID)
}

// ManualAssign — the admin picked a specific agent from the
// nearby list. No distance ranking, but same bookkeeping.
func ManualAssign(ctx context.Context, pool *pgxpool.Pool, orderID, agentID string) (interface{}, error) {
	var status string
	err := pool.QueryRow(ctx,
		`SELECT status::text FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&status)
	if err != nil {
		return nil, fmt.Errorf("order not found")
	}
	if status == "DELIVERED" || status == "CANCELLED" {
		return nil, fmt.Errorf("cannot assign a %s order", status)
	}

	profile, err := repository.GetAgentProfile(ctx, pool, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found")
	}

	if err := repository.AssignAgentToOrder(ctx, pool, orderID, agentID); err != nil {
		return nil, err
	}
	if err := repository.InsertStatusHistory(ctx, pool, orderID, "ASSIGNED", "ADMIN", agentID, "manual assignment"); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"order_id": orderID,
		"status":   "ASSIGNED",
		"assigned_agent": map[string]interface{}{
			"id":   profile.UserID,
			"name": profile.Name,
		},
	}, nil
}

// ---------------------------------------------------------
// Status updates — the §6 state machine is the source of
// truth. Agents and customers must follow it; an admin may
// jump anywhere, but every transition still writes history.
// ---------------------------------------------------------

var legalTransitions = map[string][]string{
	"CREATED":          {"CONFIRMED", "CANCELLED"},
	"CONFIRMED":        {"ASSIGNED", "CANCELLED"},
	"ASSIGNED":         {"PICKED_UP", "CANCELLED"},
	"PICKED_UP":        {"IN_TRANSIT"},
	"IN_TRANSIT":       {"OUT_FOR_DELIVERY"},
	"OUT_FOR_DELIVERY": {"DELIVERED", "FAILED"},
	"FAILED":           {},
	"RESCHEDULED":      {"ASSIGNED"},
	"DELIVERED":        {},
	"CANCELLED":        {},
}

func canTransition(from, to string) bool {
	for _, next := range legalTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

func isValidStatus(s string) bool {
	_, ok := legalTransitions[s]
	return ok
}

type UpdateStatusInput struct {
	Status    string `json:"status"`
	Notes     string `json:"notes"`
	Role      string // from the token, never the body
	ActorID   string // from the token
}

type UpdateStatusResult struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func UpdateStatus(ctx context.Context, pool *pgxpool.Pool, orderID string, in *UpdateStatusInput) (*UpdateStatusResult, error) {
	current, err := repository.GetCurrentOrderStatus(ctx, pool, orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found")
	}
	if !isValidStatus(in.Status) {
		return nil, fmt.Errorf("unknown status %q", in.Status)
	}

	actorType := "CUSTOMER"
	switch in.Role {
	case "admin":
		actorType = "ADMIN"
	case "agent":
		actorType = "AGENT"
	}

	// Admin overrides skip the state machine; everyone else follows it.
	if actorType != "ADMIN" && !canTransition(current, in.Status) {
		return nil, fmt.Errorf("illegal transition %s → %s", current, in.Status)
	}
	if in.Status == "CANCELLED" && current != "CREATED" && current != "CONFIRMED" && actorType == "CUSTOMER" {
		return nil, fmt.Errorf("customers can only cancel before pickup")
	}

	if in.Status == "DELIVERED" && current == "OUT_FOR_DELIVERY" {
		// Keep assigned_agent_id as the delivery record; just free the agent.
		var agentID *string
		if err := pool.QueryRow(ctx,
			`SELECT assigned_agent_id FROM orders WHERE id = $1`, orderID).Scan(&agentID); err == nil && agentID != nil {
			_ = repository.FreeAgent(ctx, pool, *agentID)
		}
	} else if in.Status == "RESCHEDULED" || in.Status == "CANCELLED" {
		agentID, err := clearAssignedAgent(ctx, pool, orderID)
		if err != nil {
			return nil, err
		}
		if agentID != "" {
			_ = repository.FreeAgent(ctx, pool, agentID)
		}
	}

	if err := repository.SetOrderStatus(ctx, pool, orderID, in.Status); err != nil {
		return nil, err
	}

	notes := in.Notes
	if notes == "" {
		notes = fmt.Sprintf("%s → %s", current, in.Status)
	}
	if err := repository.InsertStatusHistory(ctx, pool, orderID, in.Status, actorType, in.ActorID, notes); err != nil {
		return nil, err
	}

	notifyCustomer(ctx, pool, orderID, in.Status)

	return &UpdateStatusResult{OrderID: orderID, Status: in.Status}, nil
}

// clearAssignedAgent returns (and detaches) the current agent so the
// DELIVERED / RESCHEDULED / CANCELLED side effects run atomically.
func clearAssignedAgent(ctx context.Context, pool *pgxpool.Pool, orderID string) (string, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var agentID *string
	err = tx.QueryRow(ctx,
		`SELECT assigned_agent_id FROM orders WHERE id = $1 FOR UPDATE`, orderID).Scan(&agentID)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE orders SET assigned_agent_id = NULL, updated_at = now() WHERE id = $1`, orderID); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	if agentID == nil {
		return "", nil
	}
	return *agentID, nil
}

// notifyCustomer is best-effort: a failed send is logged in the
// notifications table by the Notifier itself, never blocks the update.
func notifyCustomer(ctx context.Context, pool *pgxpool.Pool, orderID, status string) {
	var customerID string
	err := pool.QueryRow(ctx,
		`SELECT customer_id FROM orders WHERE id = $1`, orderID).Scan(&customerID)
	if err != nil {
		return
	}
	_ = NewNotifier(pool).SendOrderUpdate(ctx, orderID, customerID, status)
}

// ---------------------------------------------------------
// Reschedule — only a FAILED order can be rescheduled. The
// request row links old agent → new agent, the order goes
// RESCHEDULED with a future date, and auto-assignment runs
// again for the new date (best-effort).
// ---------------------------------------------------------

type RescheduleResult struct {
	OrderID              string `json:"order_id"`
	Status               string `json:"status"`
	ScheduledDeliveryDate string `json:"scheduled_delivery_date"`
	AssignedAgent        *string `json:"assigned_agent,omitempty"`
}

func Reschedule(ctx context.Context, pool *pgxpool.Pool, orderID, requestedDate string) (*RescheduleResult, error) {
	current, err := repository.GetCurrentOrderStatus(ctx, pool, orderID)
	if err != nil {
		return nil, fmt.Errorf("order not found")
	}
	if current != "FAILED" {
		return nil, fmt.Errorf("only FAILED deliveries can be rescheduled (current: %s)", current)
	}

	date, err := time.Parse("2006-01-02", requestedDate)
	if err != nil {
		return nil, fmt.Errorf("requested_date must be YYYY-MM-DD")
	}
	if !date.After(time.Now()) {
		return nil, fmt.Errorf("requested_date must be in the future")
	}

	attemptNo, err := repository.CountRescheduleAttempts(ctx, pool, orderID)
	if err != nil {
		return nil, err
	}

	prevAgentID, _ := clearAssignedAgent(ctx, pool, orderID)
	if prevAgentID != "" {
		_ = repository.FreeAgent(ctx, pool, prevAgentID)
	}

	failureReason := latestHistoryNotes(ctx, pool, orderID, "FAILED")
	if err := repository.InsertRescheduleRequest(ctx, pool, orderID, attemptNo+1, failureReason, prevAgentID, date.Format("2006-01-02")); err != nil {
		return nil, err
	}
	if err := repository.SetScheduledDeliveryDate(ctx, pool, orderID, date); err != nil {
		return nil, err
	}
	if err := repository.SetOrderStatus(ctx, pool, orderID, "RESCHEDULED"); err != nil {
		return nil, err
	}
	if err := repository.InsertStatusHistory(ctx, pool, orderID, "RESCHEDULED", "CUSTOMER", "", "rescheduled for "+date.Format("2006-01-02")); err != nil {
		return nil, err
	}

	result := &RescheduleResult{
		OrderID:               orderID,
		Status:                "RESCHEDULED",
		ScheduledDeliveryDate: date.Format("2006-01-02"),
	}

	// Re-run auto-assignment for the new date; if nobody is free the
	// order stays RESCHEDULED for admin manual assignment.
	if _, err := AssignAgent(ctx, pool, orderID); err == nil {
		assigned := ""
		_ = pool.QueryRow(ctx,
			`SELECT assigned_agent_id::text FROM orders WHERE id = $1`, orderID).Scan(&assigned)
		result.AssignedAgent = &assigned
	}

	notifyCustomer(ctx, pool, orderID, "RESCHEDULED")
	return result, nil
}

func latestHistoryNotes(ctx context.Context, pool *pgxpool.Pool, orderID, status string) string {
	var notes *string
	_ = pool.QueryRow(ctx,
		`SELECT notes FROM order_status_history
		 WHERE order_id = $1 AND status::text = $2 ORDER BY created_at DESC LIMIT 1`,
		orderID, status).Scan(&notes)
	if notes == nil {
		return ""
	}
	return *notes
}
