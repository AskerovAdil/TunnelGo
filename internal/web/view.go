package web

import (
	"errors"

	"github.com/tunnelgo/tunnelgo/internal/domain"
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
}

type DashboardData struct {
	Clients []ClientRow
}

type CreateResultData struct {
	Error  string
	Name   string
	ID     string
	Config string
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

func CreateResultFromService(res *service.CreateResult, err error) CreateResultData {
	if err != nil {
		return CreateResultData{Error: UserMessage(err)}
	}
	return CreateResultData{
		Name:   res.Client.Name,
		ID:     res.Client.ID.String(),
		Config: res.Config,
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
		return "Failed to create client. Check server logs."
	}
}
