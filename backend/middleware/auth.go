package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// AuthSecret signs and verifies the first-party session tokens
// (HS256). Set AUTH_SECRET in production; the fallback exists so a
// local dev run doesn't crash.
func AuthSecret() []byte {
	if s := os.Getenv("AUTH_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("dev-secret-change-me")
}

// RequireAuth verifies the Bearer token issued by /api/auth/login,
// then exposes user_id + role to handlers via c.Locals. Roles come
// from the users table at login time — app-level RBAC per the schema.
func RequireAuth() fiber.Handler {
	return func(c fiber.Ctx) error {
		auth := c.Get("Authorization")
		if len(auth) < 8 || !strings.EqualFold(auth[:7], "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing bearer token"})
		}

		token, err := jwt.Parse([]byte(auth[7:]),
			jwt.WithKey(jwa.HS256, AuthSecret()),
			jwt.WithValidate(true))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired token"})
		}

		roleClaim, _ := token.Get("role")
		role, _ := roleClaim.(string)
		c.Locals("user_id", token.Subject())
		c.Locals("role", role)

		return c.Next()
	}
}

func RequireRole(allowed ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		for _, a := range allowed {
			if role == a {
				return c.Next()
			}
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "forbidden: requires role " + strings.Join(allowed, " or ")})
	}
}
