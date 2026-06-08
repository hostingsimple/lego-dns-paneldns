// Package paneldns implements a lego DNS provider for PanelDNS.
// https://github.com/Veeau/lego-dns-paneldns
package paneldns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/platform/config/env"
)

const (
	envAPIURL   = "PANELDNS_URL"
	envAPIToken = "PANELDNS_TOKEN"
	envTTL      = "PANELDNS_TTL"

	defaultAPIURL             = "https://app.paneldns.com"
	defaultTTL                = 60
	defaultHTTPTimeout        = 30 * time.Second
	defaultPropagationTimeout = 120 * time.Second
	defaultPollingInterval    = 5 * time.Second
)

// Config holds the DNS provider configuration.
type Config struct {
	APIURL             string
	APIToken           string
	TTL                int
	PropagationTimeout time.Duration
	PollingInterval    time.Duration
	HTTPClient         *http.Client
}

// NewDefaultConfig returns a Config populated from environment variables.
func NewDefaultConfig() *Config {
	return &Config{
		APIURL:             env.GetOrDefaultString(envAPIURL, defaultAPIURL),
		APIToken:           os.Getenv(envAPIToken),
		TTL:                env.GetOrDefaultInt(envTTL, defaultTTL),
		PropagationTimeout: env.GetOrDefaultSecond("PANELDNS_PROPAGATION_TIMEOUT", defaultPropagationTimeout),
		PollingInterval:    env.GetOrDefaultSecond("PANELDNS_POLLING_INTERVAL", defaultPollingInterval),
		HTTPClient:         &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// DNSProvider implements challenge.Provider for PanelDNS.
type DNSProvider struct {
	config *Config
}

// NewDNSProvider returns a DNSProvider using environment variables.
func NewDNSProvider() (*DNSProvider, error) {
	cfg := NewDefaultConfig()
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("paneldns: %s environment variable is required", envAPIToken)
	}
	return NewDNSProviderConfig(cfg)
}

// NewDNSProviderConfig returns a DNSProvider from an explicit Config.
func NewDNSProviderConfig(config *Config) (*DNSProvider, error) {
	if config == nil {
		return nil, fmt.Errorf("paneldns: config is nil")
	}
	if config.APIToken == "" {
		return nil, fmt.Errorf("paneldns: API token is required")
	}
	config.APIURL = strings.TrimRight(config.APIURL, "/")
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &DNSProvider{config: config}, nil
}

// Timeout returns propagation timeout and polling interval.
func (d *DNSProvider) Timeout() (timeout, interval time.Duration) {
	return d.config.PropagationTimeout, d.config.PollingInterval
}

// Present creates a TXT record for the DNS-01 challenge.
func (d *DNSProvider) Present(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	zoneID, zoneName, err := d.findZone(info.FQDN)
	if err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}

	fqdn := strings.TrimSuffix(info.FQDN, ".")
	recordName := strings.TrimSuffix(fqdn, "."+zoneName)

	if err := d.createRecord(zoneID, recordName, info.Value); err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}
	return nil
}

// CleanUp removes the TXT record after validation.
func (d *DNSProvider) CleanUp(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)

	zoneID, _, err := d.findZone(info.FQDN)
	if err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}

	recordID, err := d.findRecord(zoneID, info.Value)
	if err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}
	if recordID == 0 {
		return nil // already gone
	}

	if err := d.deleteRecord(zoneID, recordID); err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}
	return nil
}

// ExecPresent creates a TXT record directly from FQDN + value (exec provider mode).
func (d *DNSProvider) ExecPresent(fqdn, value string) error {
	zoneID, zoneName, err := d.findZone(fqdn + ".")
	if err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}
	clean := strings.TrimSuffix(fqdn, ".")
	recordName := strings.TrimSuffix(clean, "."+zoneName)
	if err := d.createRecord(zoneID, recordName, value); err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}
	return nil
}

// ExecCleanup removes a TXT record directly from FQDN + value (exec provider mode).
func (d *DNSProvider) ExecCleanup(fqdn, value string) error {
	zoneID, _, err := d.findZone(fqdn + ".")
	if err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}
	recordID, err := d.findRecord(zoneID, value)
	if err != nil {
		return fmt.Errorf("paneldns: %w", err)
	}
	if recordID == 0 {
		return nil
	}
	return d.deleteRecord(zoneID, recordID)
}

// ── API types ─────────────────────────────────────────────────────────────────

type apiResponse struct {
	OK   bool            `json:"ok"`
	Data json.RawMessage `json:"data"`
}

type zone struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type record struct {
	ID      int    `json:"id"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type createRecordRequest struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

// ── API helpers ───────────────────────────────────────────────────────────────

func (d *DNSProvider) findZone(fqdn string) (int, string, error) {
	labels := strings.Split(strings.TrimSuffix(fqdn, "."), ".")
	// Start from index 1 to skip "_acme-challenge"
	for i := 1; i < len(labels); i++ {
		candidate := strings.Join(labels[i:], ".")
		if candidate == "" {
			continue
		}

		zones, err := d.listZones(candidate)
		if err != nil {
			return 0, "", err
		}
		for _, z := range zones {
			if z.Name == candidate {
				return z.ID, z.Name, nil
			}
		}
	}
	return 0, "", fmt.Errorf("zone not found for %q — ensure the zone exists in PanelDNS", fqdn)
}

func (d *DNSProvider) listZones(name string) ([]zone, error) {
	endpoint := fmt.Sprintf("%s/api/v1/zones?name=%s", d.config.APIURL, url.QueryEscape(name))

	var envelope struct {
		Data []zone `json:"data"`
	}
	if err := d.doRequest(http.MethodGet, endpoint, nil, &envelope); err != nil {
		return nil, err
	}
	return envelope.Data, nil
}

func (d *DNSProvider) createRecord(zoneID int, name, content string) error {
	endpoint := fmt.Sprintf("%s/api/v1/zones/%d/records", d.config.APIURL, zoneID)
	body := createRecordRequest{Type: "TXT", Name: name, Content: content, TTL: d.config.TTL}
	return d.doRequest(http.MethodPost, endpoint, body, nil)
}

func (d *DNSProvider) findRecord(zoneID int, content string) (int, error) {
	endpoint := fmt.Sprintf("%s/api/v1/zones/%d/records", d.config.APIURL, zoneID)

	var envelope struct {
		Data []record `json:"data"`
	}
	if err := d.doRequest(http.MethodGet, endpoint, nil, &envelope); err != nil {
		return 0, err
	}
	for _, r := range envelope.Data {
		if r.Type == "TXT" && r.Content == content {
			return r.ID, nil
		}
	}
	return 0, nil
}

func (d *DNSProvider) deleteRecord(zoneID, recordID int) error {
	endpoint := fmt.Sprintf("%s/api/v1/zones/%d/records/%d", d.config.APIURL, zoneID, recordID)
	return d.doRequest(http.MethodDelete, endpoint, nil, nil)
}

func (d *DNSProvider) doRequest(method, endpoint string, body, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, endpoint, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+d.config.APIToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := d.config.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		return json.Unmarshal(respBody, result)
	}
	return nil
}
