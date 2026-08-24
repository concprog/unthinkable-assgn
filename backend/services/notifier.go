package services

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/resend/resend-go/v3"

	"lastmile-tracker/backend/repository"
)

type Notifier struct {
	pool *pgxpool.Pool
}

func NewNotifier(pool *pgxpool.Pool) *Notifier {
	return &Notifier{pool: pool}
}

// EmailPayload is what the backend renders into an email. RESEND_API_KEY
// and EMAIL_FROM are placeholder-friendly: when unset the send fails
// cleanly and the notification lands as FAILED, retryable from the
// admin resend endpoint.
type EmailPayload struct {
	NotificationID string
	Kind           string // order_status | verify_email
	To             string
	CustomerName   string
	OrderNumber    string
	Status         string
	VerifyURL      string
}

var emailClient *resend.Client

func resendClient() (*resend.Client, error) {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" || apiKey == "re_xxxxxxxxx" {
		return nil, fmt.Errorf("RESEND_API_KEY is not set (email service not configured)")
	}
	if emailClient == nil {
		emailClient = resend.NewClient(apiKey)
	}
	return emailClient, nil
}

func emailFrom() (string, error) {
	from := os.Getenv("EMAIL_FROM")
	if from == "" {
		return "", fmt.Errorf("EMAIL_FROM is not set")
	}
	return from, nil
}

// Send delivers the payload via Resend with an HTML template per kind.
func (p *EmailPayload) Send(ctx context.Context) error {
	client, err := resendClient()
	if err != nil {
		return err
	}
	from, err := emailFrom()
	if err != nil {
		return err
	}

	subject, html := p.render()
	params := &resend.SendEmailRequest{
		From:    from,
		To:      []string{p.To},
		Subject: subject,
		Html:    html,
	}

	sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	_, err = client.Emails.SendWithContext(sendCtx, params)
	return err
}

func (p *EmailPayload) render() (string, string) {
	switch p.Kind {
	case "verify_email":
		return "Verify your Last-Mile Tracker account", renderVerify(p.CustomerName, p.VerifyURL)
	default:
		subject := "Order status update"
		if p.OrderNumber != "" {
			subject = fmt.Sprintf("Order %s update: %s", p.OrderNumber, humanizeStatus(p.Status))
		}
		return subject, renderOrderStatus(p.CustomerName, p.OrderNumber, p.Status)
	}
}

// SendVerificationEmail is used by POST /api/auth/send-verification.
func SendVerificationEmail(ctx context.Context, to, name, verifyURL string) error {
	payload := &EmailPayload{
		Kind:         "verify_email",
		To:           to,
		CustomerName: name,
		VerifyURL:    verifyURL,
	}
	return payload.Send(ctx)
}

func (n *Notifier) SendOrderUpdate(ctx context.Context, orderID, userID, triggerStatus string) error {
	notificationID, err := repository.QueueNotification(ctx, n.pool, orderID, userID, "EMAIL", triggerStatus)
	if err != nil {
		return err
	}

	err = n.deliver(ctx, notificationID)
	if err != nil {
		return repository.MarkNotificationFailed(ctx, n.pool, notificationID, err.Error())
	}
	return repository.MarkNotificationSent(ctx, n.pool, notificationID)
}

// ResendNotification retries a FAILED/PENDING notification row — used
// by POST /api/admin/notifications/:id/resend.
func (n *Notifier) ResendNotification(ctx context.Context, notificationID string) error {
	err := n.deliver(ctx, notificationID)
	if err != nil {
		return repository.MarkNotificationFailed(ctx, n.pool, notificationID, err.Error())
	}
	return repository.MarkNotificationSent(ctx, n.pool, notificationID)
}

// deliver resolves recipient + order from the queued notification row,
// then hands it to Resend.
func (n *Notifier) deliver(ctx context.Context, notificationID string) error {
	row, err := repository.GetNotificationContext(ctx, n.pool, notificationID)
	if err != nil {
		return err
	}

	payload := &EmailPayload{
		NotificationID: notificationID,
		Kind:           "order_status",
		To:             row.Email,
		CustomerName:   row.CustomerName,
		OrderNumber:    row.OrderNumber,
		Status:         row.TriggerStatus,
	}
	err = payload.Send(ctx)
	if err != nil {
		return fmt.Errorf("order %s to %s: %w", row.OrderNumber, row.Email, err)
	}
	return nil
}

func humanizeStatus(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '_' {
			out = append(out, ' ')
			continue
		}
		if i == 0 || s[i-1] == '_' {
			if ch >= 'a' && ch <= 'z' {
				ch -= 32
			}
		} else if ch >= 'A' && ch <= 'Z' {
			ch += 32
		}
		out = append(out, ch)
	}
	return string(out)
}

// ---------------------------------------------------------
// HTML templates (kept inline — simple layouts, no deps)
// ---------------------------------------------------------

const emailShell = `<!doctype html>
<html><body style="margin:0;padding:24px;background:#f4f4f5;font-family:Arial,Helvetica,sans-serif;color:#18181b;">
  <div style="max-width:520px;margin:0 auto;background:#ffffff;border-radius:8px;border:1px solid #e4e4e7;overflow:hidden;">
    <div style="padding:16px 24px;border-bottom:1px solid #e4e4e7;font-weight:bold;">Last-Mile Tracker</div>
    <div style="padding:24px;">%s</div>
    <div style="padding:12px 24px;border-top:1px solid #e4e4e7;color:#a1a1aa;font-size:11px;">
      Automated message from Last-Mile Delivery Tracker &mdash; please do not reply.
    </div>
  </div>
</body></html>`

var statusCopy = map[string]string{
	"CREATED":          "Your order has been created.",
	"CONFIRMED":        "Your order has been confirmed.",
	"ASSIGNED":         "A delivery agent has been assigned to your order.",
	"PICKED_UP":        "Your package has been picked up.",
	"IN_TRANSIT":       "Your package is in transit.",
	"OUT_FOR_DELIVERY": "Your package is out for delivery today.",
	"DELIVERED":        "Your package has been delivered. Thank you!",
	"FAILED":           "Delivery attempt failed. You can reschedule a new attempt from your dashboard.",
	"RESCHEDULED":      "Your delivery has been rescheduled.",
	"CANCELLED":        "Your order has been cancelled.",
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;")
	return r.Replace(s)
}

func renderOrderStatus(name, orderNumber, status string) string {
	copyText, ok := statusCopy[status]
	if !ok {
		copyText = fmt.Sprintf("Your order status changed to %s.", humanizeStatus(status))
	}
	body := fmt.Sprintf(`
      <h2 style="margin:0 0 12px;font-size:18px;">Update on order %s</h2>
      <p style="margin:0 0 12px;">Hi %s,</p>
      <p style="margin:0 0 16px;">%s</p>
      <p style="margin:0;font-size:13px;color:#52525b;">Order:
        <strong>%s</strong> &middot; Status: <strong>%s</strong></p>`,
		htmlEscape(orderNumber), htmlEscape(name), copyText,
		htmlEscape(orderNumber), htmlEscape(humanizeStatus(status)))
	return fmt.Sprintf(emailShell, body)
}

func renderVerify(name, verifyURL string) string {
	body := fmt.Sprintf(`
      <h2 style="margin:0 0 12px;font-size:18px;">Verify your email</h2>
      <p style="margin:0 0 12px;">Hi %s,</p>
      <p style="margin:0 0 20px;">Confirm your email address to finish setting up your account.</p>
      <a href="%s" style="display:inline-block;background:#18181b;color:#ffffff;text-decoration:none;padding:10px 18px;border-radius:6px;font-size:14px;">Verify email</a>
      <p style="margin:16px 0 0;font-size:12px;color:#71717a;">This link expires in 24 hours.</p>`,
		htmlEscape(name), verifyURL)
	return fmt.Sprintf(emailShell, body)
}
