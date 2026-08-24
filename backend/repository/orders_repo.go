package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentCandidateRow struct {
	UserID string
	Name   string
	Lat    float64
	Lng    float64
}

func GetOrderPickupInfo(ctx context.Context, pool *pgxpool.Pool, orderID string) (int, float64, float64, error) {
	var zoneID int
	var lat, lng float64
	err := pool.QueryRow(ctx,
		`SELECT o.pickup_zone_id, a.latitude, a.longitude
		 FROM orders o JOIN addresses a ON a.id = o.pickup_address_id
		 WHERE o.id = $1`, orderID).Scan(&zoneID, &lat, &lng)
	if err != nil {
		return 0, 0, 0, pgx.ErrNoRows
	}
	return zoneID, lat, lng, nil
}

func ListAvailableAgents(ctx context.Context, pool *pgxpool.Pool, zoneID int) ([]AgentCandidateRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT ap.user_id, u.full_name, ap.current_latitude, ap.current_longitude
		 FROM agent_profiles ap JOIN users u ON u.id = ap.user_id
		 WHERE ap.zone_id = $1 AND ap.availability = 'AVAILABLE' AND u.is_active = true`,
		zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []AgentCandidateRow
	for rows.Next() {
		var a AgentCandidateRow
		if err := rows.Scan(&a.UserID, &a.Name, &a.Lat, &a.Lng); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func AssignAgentToOrder(ctx context.Context, pool *pgxpool.Pool, orderID, agentID string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE orders SET assigned_agent_id = $1, status = 'ASSIGNED', updated_at = now() WHERE id = $2`,
		agentID, orderID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE agent_profiles SET availability = 'BUSY', updated_at = now() WHERE user_id = $1`,
		agentID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// DBTX is satisfied by both *pgxpool.Pool and pgx.Tx, so history rows can
// be written inside a caller's transaction or standalone.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func InsertStatusHistory(ctx context.Context, db DBTX, orderID, status, actorType, actorID, notes string) error {
	var actorArg interface{}
	if actorID != "" {
		actorArg = actorID
	}
	var notesArg interface{}
	if notes != "" {
		notesArg = notes
	}
	_, err := db.Exec(ctx,
		`INSERT INTO order_status_history (order_id, status, actor_type, actor_id, notes)
		 VALUES ($1, $2, $3::actor_type, $4, $5)`,
		orderID, status, actorType, actorArg, notesArg)
	return err
}

type OrderRow struct {
	ID          string
	OrderNumber string
	Status      string
	CreatedAt   time.Time
}

func GetOrderByID(ctx context.Context, pool *pgxpool.Pool, id string) (*OrderRow, error) {
	row := &OrderRow{}
	err := pool.QueryRow(ctx,
		`SELECT id, order_number, status, created_at FROM orders WHERE id = $1`, id).
		Scan(&row.ID, &row.OrderNumber, &row.Status, &row.CreatedAt)
	if err != nil {
		return nil, pgx.ErrNoRows
	}
	return row, nil
}

// ---------------------------------------------------------
// Order list + detail (customer dashboard / agent app)
// ---------------------------------------------------------

type OrderSummaryRow struct {
	ID          string    `json:"id"`
	OrderNumber string    `json:"order_number"`
	Status      string    `json:"status"`
	TotalCharge float64   `json:"total_charge"`
	CreatedAt   time.Time `json:"created_at"`
}

func ListOrdersForUser(ctx context.Context, pool *pgxpool.Pool, userID string) ([]OrderSummaryRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, order_number, status::text, total_charge, created_at
		 FROM orders
		 WHERE customer_id = $1 OR created_by_id = $1
		 ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []OrderSummaryRow
	for rows.Next() {
		var o OrderSummaryRow
		if err := rows.Scan(&o.ID, &o.OrderNumber, &o.Status, &o.TotalCharge, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

type AdminOrderRow struct {
	ID               string  `json:"id"`
	OrderNumber      string  `json:"order_number"`
	OrderType        string  `json:"order_type"`
	Status           string  `json:"status"`
	PickupZoneID     int     `json:"pickup_zone_id"`
	AssignedAgentName *string `json:"assigned_agent_name"`
}

type AdminOrderFilter struct {
	Zone     string   // numeric zone id, or a letter matched against "Zone X" names
	Statuses []string // zero or more repeated ?status= values
	AgentID  string   // assigned-agent UUID
}

func ListAdminOrders(ctx context.Context, pool *pgxpool.Pool, f AdminOrderFilter) ([]AdminOrderRow, error) {
	where := "WHERE TRUE"
	args := []interface{}{}

	if len(f.Statuses) > 0 {
		args = append(args, f.Statuses)
		where += fmt.Sprintf(" AND o.status::text = ANY($%d)", len(args))
	}
	if f.Zone != "" {
		if isDigits(f.Zone) {
			args = append(args, f.Zone)
			where += fmt.Sprintf(" AND o.pickup_zone_id = $%d", len(args))
		} else {
			args = append(args, f.Zone)
			where += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM zones z WHERE z.id = o.pickup_zone_id
				AND z.name ILIKE 'zone ' || $%d || '%%')`, len(args))
		}
	}
	if f.AgentID != "" {
		args = append(args, f.AgentID)
		where += fmt.Sprintf(" AND o.assigned_agent_id = $%d", len(args))
	}

	rows, err := pool.Query(ctx,
		`SELECT o.id, o.order_number, o.order_type::text, o.status::text, o.pickup_zone_id, a.full_name
		 FROM orders o LEFT JOIN users a ON a.id = o.assigned_agent_id
		 `+where+`
		 ORDER BY o.created_at DESC LIMIT 200`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []AdminOrderRow
	for rows.Next() {
		var o AdminOrderRow
		if err := rows.Scan(&o.ID, &o.OrderNumber, &o.OrderType, &o.Status, &o.PickupZoneID, &o.AssignedAgentName); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// ---------------------------------------------------------
// Full order detail — one query joins everything the
// customer tracking page and the agent action page render.
// ---------------------------------------------------------

type OrderDetailRow struct {
	ID              string
	OrderNumber     string
	Status          string
	PaymentType     string
	OrderValue      *float64
	AssignedAgentID *string

	BaseCharge    float64
	CODSurcharge  float64
	FuelSurcharge float64
	GSTAmount     float64
	TotalCharge   float64

	CustomerName string
	DropLine1    string
	DropCity     string
	DropPincode  string

	CreatedAt time.Time
}

type StatusHistoryRow struct {
	Status    string    `json:"status"`
	Notes     *string   `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func GetOrderDetail(ctx context.Context, pool *pgxpool.Pool, id string) (*OrderDetailRow, error) {
	row := &OrderDetailRow{}
	err := pool.QueryRow(ctx,
		`SELECT o.id, o.order_number, o.status::text, o.payment_type::text, o.order_value,
		        o.assigned_agent_id,
		        o.base_charge, o.cod_surcharge, o.fuel_surcharge, o.gst_amount, o.total_charge,
		        cu.full_name, da.line1, da.city, da.pincode, o.created_at
		 FROM orders o
		 JOIN users cu ON cu.id = o.customer_id
		 JOIN addresses da ON da.id = o.drop_address_id
		 WHERE o.id = $1`, id).
		Scan(&row.ID, &row.OrderNumber, &row.Status, &row.PaymentType, &row.OrderValue,
			&row.AssignedAgentID,
			&row.BaseCharge, &row.CODSurcharge, &row.FuelSurcharge, &row.GSTAmount, &row.TotalCharge,
			&row.CustomerName, &row.DropLine1, &row.DropCity, &row.DropPincode, &row.CreatedAt)
	if err != nil {
		return nil, pgx.ErrNoRows
	}
	return row, nil
}

func GetOrderHistory(ctx context.Context, pool *pgxpool.Pool, orderID string) ([]StatusHistoryRow, error) {
	rows, err := pool.Query(ctx,
		`SELECT status::text, notes, created_at
		 FROM order_status_history WHERE order_id = $1 ORDER BY created_at ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []StatusHistoryRow
	for rows.Next() {
		var h StatusHistoryRow
		if err := rows.Scan(&h.Status, &h.Notes, &h.CreatedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

func GetCurrentOrderStatus(ctx context.Context, pool *pgxpool.Pool, orderID string) (string, error) {
	var status string
	err := pool.QueryRow(ctx,
		`SELECT status::text FROM orders WHERE id = $1`, orderID).Scan(&status)
	if err != nil {
		return "", pgx.ErrNoRows
	}
	return status, nil
}

// ---------------------------------------------------------
// Order creation — addresses first (normalized), then the
// order row with the snapshotted charge breakdown.
// ---------------------------------------------------------

type InsertAddressInput struct {
	UserID  string // may be empty for admin-entered ad hoc addresses
	Line1   string
	Line2   string
	City    string
	State   string
	Pincode string
	Lat     *float64
	Lng     *float64
}

func InsertAddress(ctx context.Context, tx pgx.Tx, in *InsertAddressInput) (string, error) {
	var userArg interface{}
	if in.UserID != "" {
		userArg = in.UserID
	}
	var line2Arg interface{}
	if in.Line2 != "" {
		line2Arg = in.Line2
	}
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO addresses (user_id, line1, line2, city, state, pincode, latitude, longitude)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		userArg, in.Line1, line2Arg, in.City, in.State, in.Pincode, in.Lat, in.Lng).Scan(&id)
	return id, err
}

type InsertOrderInput struct {
	CustomerID       string
	CreatedByID      string
	PickupAddressID  string
	DropAddressID    string
	PickupZoneID     int
	DropZoneID       int
	OrderType        string
	PaymentType      string
	LengthCM         float64
	BreadthCM        float64
	HeightCM         float64
	ActualWeightKG   float64
	VolumetricWeight float64
	ChargeableWeight float64
	RateCardID       int
	BaseCharge       float64
	CODSurcharge     float64
	FuelSurcharge    float64
	GSTAmount        float64
	TotalCharge      float64
	OrderValue       *float64
}

// CreateOrder persists the order inside the caller's transaction and returns its ID.
// The human-readable order number is generated here: LMD-YYYYMMDD-NNNN, where NNNN
// counts the orders created today (retried on unique-violation from concurrent inserts).
func CreateOrder(ctx context.Context, tx pgx.Tx, in *InsertOrderInput) (string, error) {
	var orderValue interface{}
	if in.OrderValue != nil {
		orderValue = *in.OrderValue
	}

	var id, orderNumber string
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		seq, seqErr := todaysOrderSeq(ctx, tx)
		if seqErr != nil {
			return "", seqErr
		}
		orderNumber = fmt.Sprintf("LMD-%s-%04d", time.Now().Format("20060102"), seq)

		err = tx.QueryRow(ctx,
			`INSERT INTO orders (
				order_number, customer_id, created_by_id,
				pickup_address_id, drop_address_id, pickup_zone_id, drop_zone_id,
				order_type, payment_type,
				length_cm, breadth_cm, height_cm, actual_weight_kg,
				volumetric_weight_kg, chargeable_weight_kg,
				rate_card_id, base_charge, cod_surcharge, fuel_surcharge, gst_amount, total_charge,
				order_value, status
			 ) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,'CREATED')
			 RETURNING id`,
			orderNumber, in.CustomerID, in.CreatedByID,
			in.PickupAddressID, in.DropAddressID, in.PickupZoneID, in.DropZoneID,
			in.OrderType, in.PaymentType,
			in.LengthCM, in.BreadthCM, in.HeightCM, in.ActualWeightKG,
			in.VolumetricWeight, in.ChargeableWeight,
			in.RateCardID, in.BaseCharge, in.CODSurcharge, in.FuelSurcharge, in.GSTAmount, in.TotalCharge,
			orderValue).Scan(&id)
		if err == nil {
			return id, nil
		}
		if !isUniqueViolation(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate a unique order number")
}

func todaysOrderSeq(ctx context.Context, tx pgx.Tx) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT count(*) FROM orders WHERE created_at >= date_trunc('day', now())`).Scan(&n)
	return n + 1, err
}

var ErrUniqueViolation = errors.New("unique constraint violated")

func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

// ---------------------------------------------------------
// Status transitions + reschedule writes
// ---------------------------------------------------------

// SetOrderStatus applies a plain status change plus the side effects each
// transition carries. All in one statement so status and its consequences
// can never drift apart.
func SetOrderStatus(ctx context.Context, pool *pgxpool.Pool, orderID, status string) error {
	tag, err := pool.Exec(ctx,
		`UPDATE orders SET
			status = $2::order_status,
			delivered_at = CASE WHEN $2 = 'DELIVERED' THEN now() ELSE delivered_at END,
			assigned_agent_id = CASE WHEN $2 IN ('RESCHEDULED','CANCELLED') THEN NULL ELSE assigned_agent_id END,
			scheduled_delivery_date = CASE WHEN $2 = 'RESCHEDULED' THEN scheduled_delivery_date ELSE scheduled_delivery_date END,
			updated_at = now()
		 WHERE id = $1`, orderID, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func FreeAgent(ctx context.Context, pool *pgxpool.Pool, agentID string) error {
	_, err := pool.Exec(ctx,
		`UPDATE agent_profiles SET availability = 'AVAILABLE', updated_at = now() WHERE user_id = $1`,
		agentID)
	return err
}

func SetScheduledDeliveryDate(ctx context.Context, pool *pgxpool.Pool, orderID string, date time.Time) error {
	_, err := pool.Exec(ctx,
		`UPDATE orders SET scheduled_delivery_date = $2, updated_at = now() WHERE id = $1`,
		orderID, date.Format("2006-01-02"))
	return err
}

func CountRescheduleAttempts(ctx context.Context, pool *pgxpool.Pool, orderID string) (int, error) {
	var n int
	err := pool.QueryRow(ctx,
		`SELECT coalesce(max(failed_attempt_no), 0) FROM reschedule_requests WHERE order_id = $1`,
		orderID).Scan(&n)
	return n, err
}

func InsertRescheduleRequest(ctx context.Context, pool *pgxpool.Pool, orderID string, attemptNo int, failureReason, previousAgentID, requestedDate string) error {
	var reasonArg, prevAgentArg interface{}
	if failureReason != "" {
		reasonArg = failureReason
	}
	if previousAgentID != "" {
		prevAgentArg = previousAgentID
	}
	_, err := pool.Exec(ctx,
		`INSERT INTO reschedule_requests (order_id, failed_attempt_no, failure_reason, previous_agent_id, requested_date)
		 VALUES ($1, $2, $3, $4, $5)`,
		orderID, attemptNo, reasonArg, prevAgentArg, requestedDate)
	return err
}
