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
