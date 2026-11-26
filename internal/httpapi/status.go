package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

type serverLiveStatus struct {
	Online          bool
	Error           string
	Version         string
	ClientCounts    int64
	CurConns        int64
	TotalTrafficIn  int64
	TotalTrafficOut int64
	ProxyTypeCounts map[string]int64
	AllowPorts      string
}

func (s serverLiveStatus) TrafficOutHuman() string {
	return formatBytes(s.TotalTrafficOut)
}

func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	val := float64(b)
	for _, u := range units {
		val /= 1024
		if val < 1024 || u == units[len(units)-1] {
			if val >= 100 {
				return fmt.Sprintf("%.0f %s", val, u)
			}
			return fmt.Sprintf("%.1f %s", val, u)
		}
	}
	return fmt.Sprintf("%d B", b)
}

func (d Deps) serverStatus(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	status := fetchServerStatus(r, s)
	if err := d.Render.RenderPartial(w, "_status.html", status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func fetchServerStatus(r *http.Request, s store.Server) serverLiveStatus {
	if s.Domain == "" {
		return serverLiveStatus{Error: "Chưa cấu hình domain để đọc trạng thái trực tiếp."}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+s.Domain+"/api/serverinfo", nil)
	if err != nil {
		return serverLiveStatus{Error: err.Error()}
	}
	req.SetBasicAuth(s.DashUser, s.DashPassword)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return serverLiveStatus{Error: "Không kết nối được FRPS dashboard."}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return serverLiveStatus{Error: fmt.Sprintf("FRPS dashboard trả về HTTP %d.", resp.StatusCode)}
	}
	var payload struct {
		Version         string           `json:"version"`
		ClientCounts    int64            `json:"clientCounts"`
		CurConns        int64            `json:"curConns"`
		TotalTrafficIn  int64            `json:"totalTrafficIn"`
		TotalTrafficOut int64            `json:"totalTrafficOut"`
		ProxyTypeCounts map[string]int64 `json:"proxyTypeCount"`
		AllowPorts      string           `json:"allowPortsStr"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return serverLiveStatus{Error: "Dữ liệu trạng thái FRPS không hợp lệ."}
	}
	return serverLiveStatus{
		Online: true, Version: payload.Version, ClientCounts: payload.ClientCounts,
		CurConns: payload.CurConns, TotalTrafficIn: payload.TotalTrafficIn,
		TotalTrafficOut: payload.TotalTrafficOut, ProxyTypeCounts: payload.ProxyTypeCounts,
		AllowPorts: payload.AllowPorts,
	}
}
