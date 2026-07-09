package handler

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/tunnelgo/tunnelgo/internal/domain"
)

func mapServiceError(err error) *echo.HTTPError {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	case errors.Is(err, domain.ErrConflict):
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, "internal server error")
	}
}
