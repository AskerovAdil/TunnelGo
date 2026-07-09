package web

import (
	"errors"
	"strings"

	"github.com/tunnelgo/tunnelgo/internal/domain"
	"github.com/tunnelgo/tunnelgo/internal/domain/client"
	"github.com/tunnelgo/tunnelgo/internal/service"
)

type ClientRow struct {
	ID            string
	Name          string
	Address       string
	CreatedAt     string
	Online        bool
	StatusLabel   string
	StatusClass   string
	LastHandshake string
	ReceiveBytes  string
	TransmitBytes string
	SSLink        string
	VLESSLink     string
}

type DashboardData struct {
	Clients []ClientRow
}

type CreateResultData struct {
	Error     string
	Name      string
	ID        string
	Config    string
	SSLink    string
	VLESSLink string
}

type LinkFns struct {
	SS    func(*client.Client) string
	VLESS func(*client.Client) string
}

func ClientRowsFromViews(views []*service.ClientView) []ClientRow {
	rows := make([]ClientRow, 0, len(views))
	for _, v := range views {
		statusLabel := "Offline"
		statusClass := "offline"
		if v.Online {
			statusLabel = "Online"
			statusClass = "online"
		}
		rows = append(rows, ClientRow{
			ID:            v.Client.ID.String(),
			Name:          v.Client.Name,
			Address:       v.Client.Address,
			CreatedAt:     FormatTime(v.Client.CreatedAt),
			Online:        v.Online,
			StatusLabel:   statusLabel,
			StatusClass:   statusClass,
			LastHandshake: FormatRelative(v.LastHandshake),
			ReceiveBytes:  FormatBytes(v.ReceiveBytes),
			TransmitBytes: FormatBytes(v.TransmitBytes),
		})
	}
	return rows
}

func ClientRowsFromViewsWithLinks(views []*service.ClientView, links LinkFns) []ClientRow {
	rows := ClientRowsFromViews(views)
	for i, v := range views {
		if links.SS != nil {
			rows[i].SSLink = links.SS(v.Client)
		}
		if links.VLESS != nil {
			rows[i].VLESSLink = links.VLESS(v.Client)
		}
	}
	return rows
}

func CreateResultFromService(res *service.CreateResult, err error) CreateResultData {
	if err != nil {
		return CreateResultData{Error: UserMessage(err)}
	}
	return CreateResultData{
		Name:      res.Client.Name,
		ID:        res.Client.ID.String(),
		Config:    res.Config,
		SSLink:    res.SSLink,
		VLESSLink: res.VLESSLink,
	}
}

func UserMessage(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		return "Invalid client name. Use letters, digits, _ and - only."
	case errors.Is(err, domain.ErrConflict):
		return "Client name already exists or the address pool is full."
	default:
		msg := err.Error()
		if strings.Contains(msg, "sync wireguard") || strings.Contains(msg, "wg syncconf") || strings.Contains(msg, "wg-quick strip") {
			return "WireGuard sync failed: " + msg
		}
		if strings.Contains(msg, "sync vless") || strings.Contains(msg, "start xray") {
			return "VLESS sync failed: " + msg
		}
		if strings.Contains(msg, "interface PrivateKey is missing") || strings.Contains(msg, "stat config") {
			return "Invalid server WireGuard config: " + msg
		}
		return msg
	}
}
