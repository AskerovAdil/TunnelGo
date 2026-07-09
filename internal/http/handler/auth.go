package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/tunnelgo/tunnelgo/internal/auth"
)

type Auth struct {
	auth *auth.Authenticator
}

func NewAuth(a *auth.Authenticator) *Auth {
	return &Auth{auth: a}
}

type loginPageData struct {
	Error string
}

func (h *Auth) LoginPage(c echo.Context) error {
	if h.isAuthenticated(c) {
		return c.Redirect(http.StatusSeeOther, "/")
	}
	return c.Render(http.StatusOK, "login.html", loginPageData{})
}

func (h *Auth) Login(c echo.Context) error {
	password := c.FormValue("password")
	if password == "" {
		return c.Render(http.StatusUnauthorized, "login.html", loginPageData{
			Error: "Invalid credentials",
		})
	}
	if !h.auth.CheckPassword(password) {
		return c.Render(http.StatusUnauthorized, "login.html", loginPageData{
			Error: "Invalid credentials",
		})
	}

	token, expiresAt, err := h.auth.CreateSessionToken()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}

	c.SetCookie(h.newSessionCookie(token, expiresAt))
	return c.Redirect(http.StatusSeeOther, "/")
}

func (h *Auth) Logout(c echo.Context) error {
	c.SetCookie(h.clearSessionCookie())
	return c.Redirect(http.StatusSeeOther, "/login")
}

func (h *Auth) Home(c echo.Context) error {
	return c.Render(http.StatusOK, "home.html", nil)
}

func (h *Auth) isAuthenticated(c echo.Context) bool {
	cookie, err := c.Cookie(h.auth.CookieName())
	if err != nil {
		return false
	}
	return h.auth.ValidateSessionToken(cookie.Value) == nil
}

func (h *Auth) newSessionCookie(token string, expiresAt time.Time) *http.Cookie {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	return &http.Cookie{
		Name:     h.auth.CookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.auth.SecureCookie(),
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
		MaxAge:   maxAge,
	}
}

func (h *Auth) clearSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     h.auth.CookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.auth.SecureCookie(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	}
}
