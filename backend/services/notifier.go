package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"lastmile-tracker/backend/repository"
)

type Notifier struct {
	pool *pgxpool.Pool
}

func NewNotifier(pool *pgxpool.Pool) *Notifier {
	return &Notifier{pool: pool}
}

func (n *Notifier) SendOrderUpdate(ctx context.Context, orderID, userID, triggerStatus string) error {
	notificationID, err := repository.QueueNotification(ctx, n.pool, orderID, userID, "EMAIL", triggerStatus)
	if err != nil {
		return err
	}

	err = sendEmail(ctx, notificationID)
	if err != nil {
		return repository.MarkNotificationFailed(ctx, n.pool, notificationID, err.Error())
	}
	return repository.MarkNotificationSent(ctx, n.pool, notificationID)
}

func sendEmail(ctx context.Context, notificationID string) error {
	apiKey := os.Getenv("EMAIL_PROVIDER_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("EMAIL_PROVIDER_API_KEY is not set")
	}

	payload, err := json.Marshal(map[string]string{
		"from":            os.Getenv("EMAIL_FROM"),
		"subject":         "Order status update",
		"notification_id": notificationID,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("email provider returned %d", resp.StatusCode)
	}
	return nil
}
