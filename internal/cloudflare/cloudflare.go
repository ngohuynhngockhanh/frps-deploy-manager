// Package cloudflare is a minimal Cloudflare API v4 client: verify a token,
// resolve a zone, and upsert A records. Used to point domains at the VPS and
// (via certbot's dns-cloudflare plugin on the remote) to issue wildcard certs.
package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const apiBase = "https://api.cloudflare.com/client/v4"

// Client talks to the Cloudflare API with a bearer token.
type Client struct {
	token string
	hc    *http.Client
}

// New returns a client for the given API token.
func New(token string) *Client {
	return &Client{token: token, hc: &http.Client{Timeout: 20 * time.Second}}
}

type apiResp struct {
	Success bool              `json:"success"`
	Errors  []json.RawMessage `json:"errors"`
	Result  json.RawMessage   `json:"result"`
}

func (c *Client) do(method, path string, body any) (json.RawMessage, error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, apiBase+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var ar apiResp
	if err := json.Unmarshal(raw, &ar); err != nil {
		return nil, fmt.Errorf("cloudflare %s %s: bad response (%d): %s", method, path, resp.StatusCode, truncate(string(raw), 300))
	}
	if !ar.Success {
		return nil, fmt.Errorf("cloudflare %s %s: %s", method, path, errString(ar.Errors))
	}
	return ar.Result, nil
}

// Verify checks the token is valid and active.
func (c *Client) Verify() error {
	_, err := c.do(http.MethodGet, "/user/tokens/verify", nil)
	return err
}

type zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ZoneID resolves the Cloudflare zone containing domain (e.g. a.vnmap3d.io.vn ->
// zone vnmap3d.io.vn), returning the zone id and zone name.
func (c *Client) ZoneID(domain string) (id, name string, err error) {
	labels := strings.Split(domain, ".")
	for i := 0; i < len(labels)-1; i++ {
		candidate := strings.Join(labels[i:], ".")
		raw, err := c.do(http.MethodGet, "/zones?name="+candidate, nil)
		if err != nil {
			return "", "", err
		}
		var zs []zone
		if err := json.Unmarshal(raw, &zs); err != nil {
			return "", "", err
		}
		if len(zs) > 0 {
			return zs[0].ID, zs[0].Name, nil
		}
	}
	return "", "", fmt.Errorf("no Cloudflare zone found for %s", domain)
}

type dnsRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

// UpsertA creates or updates an A record name -> ip in the zone. proxied=false
// keeps it DNS-only (grey cloud), required for non-HTTP frps ports.
func (c *Client) UpsertA(zoneID, name, ip string, proxied bool) error {
	raw, err := c.do(http.MethodGet, fmt.Sprintf("/zones/%s/dns_records?type=A&name=%s", zoneID, name), nil)
	if err != nil {
		return err
	}
	var existing []dnsRecord
	if err := json.Unmarshal(raw, &existing); err != nil {
		return err
	}
	rec := dnsRecord{Type: "A", Name: name, Content: ip, TTL: 120, Proxied: proxied}
	if len(existing) > 0 {
		_, err = c.do(http.MethodPut, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, existing[0].ID), rec)
		return err
	}
	_, err = c.do(http.MethodPost, fmt.Sprintf("/zones/%s/dns_records", zoneID), rec)
	return err
}

func errString(errs []json.RawMessage) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		parts = append(parts, string(e))
	}
	if len(parts) == 0 {
		return "unknown error"
	}
	return strings.Join(parts, "; ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
