package handlers

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"golang.org/x/crypto/bcrypt"

	"lastmile-tracker/backend/middleware"
	"lastmile-tracker/backend/repository"
	"lastmile-tracker/backend/services"
)

const tokenTTL = 24 * time.Hour

var allowedRoles = map[string]bool{"customer": true, "agent": true, "admin": true}

func issueToken(id, role string) (string, error) {
	tok, err := jwt.NewBuilder().
		Subject(id).
		Claim("role", role).
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(tokenTTL)).
		Build()
	if err != nil {
		return "", err
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.HS256, middleware.AuthSecret()))
	return string(signed), err
}

// Register — POST /api/auth/register {full_name, email, phone, password, role}.
// Role defaults to customer; the assignment demo allows any role at signup.
func Register(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			FullName string `json:"full_name"`
			Email    string `json:"email"`
			Phone    string `json:"phone"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.FullName == "" || req.Email == "" || req.Phone == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "full_name, email and phone are required"})
		}
		if len(req.Password) < 8 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password must be at least 8 characters"})
		}
		if req.Role == "" {
			req.Role = "customer"
		}
		if !allowedRoles[req.Role] {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role must be customer, agent or admin"})
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
		}

		id, err := repository.CreateUser(c.Context(), pool, &repository.CreateUserInput{
			FullName:     req.FullName,
			Email:        req.Email,
			Phone:        req.Phone,
			PasswordHash: string(hash),
			Role:         req.Role,
		})
		if err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
		}

		token, err := issueToken(id, req.Role)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to issue token"})
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{
			"token":          token,
			"email_verified": false,
			"user":           fiber.Map{"id": id, "role": req.Role, "full_name": req.FullName},
		})
	}
}

// Login — POST /api/auth/login {email, password} → {token, user}.
func Login(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		user, err := repository.GetUserByEmail(c.Context(), pool, strings.TrimSpace(req.Email))
		if err != nil || !user.IsActive || user.PasswordHash == nil ||
			bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)) != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid email or password"})
		}

		token, err := issueToken(user.ID, user.Role)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to issue token"})
		}
		return c.JSON(fiber.Map{
			"token":          token,
			"email_verified": user.EmailVerified,
			"user":           fiber.Map{"id": user.ID, "role": user.Role, "full_name": user.FullName},
		})
	}
}

// issueVerifyToken mints a short-lived JWT scoped to email
// verification only (purpose claim checked by VerifyEmail).
func issueVerifyToken(userID string) (string, error) {
	tok, err := jwt.NewBuilder().
		Subject(userID).
		Claim("purpose", "email_verification").
		IssuedAt(time.Now()).
		Expiration(time.Now().Add(24 * time.Hour)).
		Build()
	if err != nil {
		return "", err
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.HS256, middleware.AuthSecret()))
	return string(signed), err
}

// SendVerification — POST /api/auth/send-verification (authed): mints a
// verification token and hands it to the Next.js email service, which
// sends the actual mail via Resend. The verify link points at the
// frontend's /verify page, which calls GET /api/auth/verify.
func SendVerification(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		userID := userIDFrom(c)
		email, name, err := repository.GetUserEmail(c.Context(), pool, userID)
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
		}

		vtoken, err := issueVerifyToken(userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create verification token"})
		}

		base := os.Getenv("APP_URL") // e.g. https://frontend-production-xxx.up.railway.app
		if base == "" {
			base = "http://localhost:3000"
		}
		verifyURL := fmt.Sprintf("%s/verify?token=%s", strings.TrimRight(base, "/"), url.QueryEscape(vtoken))

		err = services.SendVerificationEmail(c.Context(), email, name, verifyURL)
		if err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "could not send verification email: " + err.Error()})
		}
		return c.JSON(fiber.Map{"ok": true})
	}
}

// VerifyEmail — GET /api/auth/verify?token=... (public): validates the
// verification JWT and marks the account verified.
func VerifyEmail(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		raw := c.Query("token")
		if raw == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "missing token"})
		}

		tok, err := jwt.Parse([]byte(raw),
			jwt.WithKey(jwa.HS256, middleware.AuthSecret()),
			jwt.WithValidate(true))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired verification token"})
		}
		purpose, _ := tok.Get("purpose")
		if p, _ := purpose.(string); p != "email_verification" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "not a verification token"})
		}

		if err := repository.SetEmailVerified(c.Context(), pool, tok.Subject()); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to verify email"})
		}
		return c.JSON(fiber.Map{"ok": true, "verified": true})
	}
}

// Me — GET /api/auth/me → the signed-in user's profile from the token subject.
func Me(pool *pgxpool.Pool) fiber.Handler {
	return func(c fiber.Ctx) error {
		user, err := repository.GetUserByID(c.Context(), pool, userIDFrom(c))
		if err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
		}
		return c.JSON(user)
	}
}
