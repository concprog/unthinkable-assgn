package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func QueueNotification(ctx context.Context, pool *pgxpool.Pool, orderID, userID, channel, triggerStatus string) (string, error) {
	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO notifications (order_id, user_id, channel, trigger_status, status)
		 VALUES ($1, $2, $3::notification_channel, $4::order_status, 'PENDING')
		 RETURNING id`,
		orderID, userID, channel, triggerStatus).Scan(&id)
	return id, err
}

func MarkNotificationSent(ctx context.Context, pool *pgxpool.Pool, notificationID string) error {
	_, err := pool.Exec(ctx,
		`UPDATE notifications SET status = 'SENT', sent_at = now() WHERE id = $1`,
		notificationID)
	return err
}

func MarkNotificationFailed(ctx context.Context, pool *pgxpool.Pool, notificationID, errMsg string) error {
	_, err := pool.Exec(ctx,
		`UPDATE notifications SET status = 'FAILED', error_message = $2 WHERE id = $1`,
		notificationID, errMsg)
	return err
}

type NotificationContextRow struct {
	ID            string
	Email         string
	CustomerName  string
	OrderNumber   string
	TriggerStatus string
	Status        string
}

// GetNotificationContext joins a queued notification to its recipient
// and order so the email service gets everything it needs in one call.
func GetNotificationContext(ctx context.Context, pool *pgxpool.Pool, notificationID string) (*NotificationContextRow, error) {
	row := &NotificationContextRow{ID: notificationID}
	err := pool.QueryRow(ctx,
		`SELECT u.email, u.full_name, o.order_number, n.trigger_status::text, n.status::text
		 FROM notifications n
		 JOIN users u ON u.id = n.user_id
		 JOIN orders o ON o.id = n.order_id
		 WHERE n.id = $1`, notificationID).
		Scan(&row.Email, &row.CustomerName, &row.OrderNumber, &row.TriggerStatus, &row.Status)
	if err != nil {
		return nil, err
	}
	return row, nil
}
