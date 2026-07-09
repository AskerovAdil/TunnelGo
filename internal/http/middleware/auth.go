package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tunnelgo/tunnelgo/internal/auth"
)

func RequireAuth(a *auth.Authenticator) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(a.CookieName())
			if err != nil || a.ValidateSessionToken(cookie.Value) != nil {
				return c.Redirect(http.StatusSeeOther, "/login")
			}
			return next(c)
		}
	}
}
