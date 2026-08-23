package middleware

import (
	"context"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v3"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

var jwksCache jwk.Set

func initJWKS() error {
	if jwksCache != nil {
		return nil
	}
	url := os.Getenv("CLERK_JWKS_URL")
	set, err := jwk.Fetch(context.Background(), url)
	if err != nil {
		return err
	}
	jwksCache = set
	return nil
}

func ClerkAuth() fiber.Handler {
	return func(c fiber.Ctx) error {
		auth := c.Get("Authorization")
		if len(auth) < 8 || auth[:7] != "Bearer " {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing bearer token"})
		}

		if err := initJWKS(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "jwks unavailable"})
		}

		token, err := jwt.Parse([]byte(auth[7:]), jwt.WithKeySet(jwksCache))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
		}

		c.Locals("user_id", token.Subject())
		c.Locals("role", token.PrivateClaims()["role"])

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
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": fmt.Sprintf("role %q not allowed", role)})
	}
}
