package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/tunnelgo/tunnelgo/internal/service"
	"github.com/tunnelgo/tunnelgo/internal/web"
)

type Client struct {
	svc *service.ClientService
}

func NewClient(svc *service.ClientService) *Client {
	return &Client{svc: svc}
}

type createResponse struct {
	Result web.CreateResultData
	List   web.DashboardData
}

func (h *Client) Dashboard(c echo.Context) error {
	views, err := h.svc.List(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	return c.Render(http.StatusOK, "dashboard.html", web.DashboardData{
		Clients: web.ClientRowsFromViews(views),
	})
}

func (h *Client) ClientListPartial(c echo.Context) error {
	views, err := h.svc.List(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}
	return c.Render(http.StatusOK, "partials/client_list.html", web.DashboardData{
		Clients: web.ClientRowsFromViews(views),
	})
}

func (h *Client) Create(c echo.Context) error {
	name := strings.TrimSpace(c.FormValue("name"))
	result, err := h.svc.Create(c.Request().Context(), name)
	if err != nil {
		slog.Warn("create client failed", "name", name, "error", err)
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		resp := createResponse{
			Result: web.CreateResultFromService(result, err),
		}
		if err == nil {
			views, listErr := h.svc.List(c.Request().Context())
			if listErr != nil {
				resp.Result = web.CreateResultData{
					Error: "Client created, but failed to refresh list. Reload the page.",
				}
			} else {
				resp.List = web.DashboardData{Clients: web.ClientRowsFromViews(views)}
			}
		}
		return c.Render(http.StatusOK, "partials/create_result.html", resp)
	}

	if err != nil {
		return mapServiceError(err)
	}
	return c.Redirect(http.StatusSeeOther, "/")
}

func (h *Client) Revoke(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid client id")
	}
	if err := h.svc.Revoke(c.Request().Context(), id); err != nil {
		if c.Request().Header.Get("HX-Request") == "true" {
			return c.String(http.StatusOK, fmt.Sprintf(`<div class="error">%s</div>`, web.UserMessage(err)))
		}
		return mapServiceError(err)
	}

	views, err := h.svc.List(c.Request().Context())
	if err != nil {
		return mapServiceError(err)
	}

	if c.Request().Header.Get("HX-Request") == "true" {
		return c.Render(http.StatusOK, "partials/client_list.html", web.DashboardData{
			Clients: web.ClientRowsFromViews(views),
		})
	}
	return c.Redirect(http.StatusSeeOther, "/")
}

func (h *Client) DownloadConfig(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid client id")
	}

	client, err := h.svc.Get(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(err)
	}

	config, err := h.svc.ClientConfig(c.Request().Context(), id)
	if err != nil {
		return mapServiceError(err)
	}

	filename := fmt.Sprintf("%s.conf", client.Name)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte(config))
}
