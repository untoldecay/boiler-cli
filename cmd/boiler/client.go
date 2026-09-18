package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// --- config (~/.boiler/config.json) ---

type Config struct {
	Server       string `json:"server"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresAt    int64  `json:"expiresAt"` // unix ms
	Email        string `json:"email,omitempty"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".boiler", "config.json")
}

func loadConfig() (*Config, error) {
	// env override for headless/CI (BOILER_SERVER + BOILER_TOKEN)
	if t := os.Getenv("BOILER_TOKEN"); t != "" {
		return &Config{Server: strings.TrimRight(os.Getenv("BOILER_SERVER"), "/"), AccessToken: t, ExpiresAt: time.Now().Add(time.Hour).UnixMilli()}, nil
	}
	b, err := os.ReadFile(configPath())
	if err != nil {
		return nil, errors.New("not logged in — run: boiler login --server <url>")
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func saveConfig(c *Config) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, b, 0o600)
}

// --- HTTP with auth + auto-refresh ---

var httpClient = &http.Client{Timeout: 120 * time.Second}

func (c *Config) ensureToken() error {
	if os.Getenv("BOILER_TOKEN") != "" {
		return nil
	}
	if c.ExpiresAt-time.Now().UnixMilli() > 30_000 {
		return nil
	}
	if c.RefreshToken == "" {
		return errors.New("session expired — run: boiler login")
	}
	// refresh
	body, _ := json.Marshal(map[string]string{"refreshToken": c.RefreshToken})
	res, err := httpClient.Post(c.Server+"/auth/refresh", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("session refresh failed — run: boiler login")
	}
	var out struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int    `json:"expiresIn"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return err
	}
	c.AccessToken = out.AccessToken
	if out.RefreshToken != "" {
		c.RefreshToken = out.RefreshToken
	}
	c.ExpiresAt = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second).UnixMilli()
	return saveConfig(c)
}

// apiRequest performs an authenticated request and returns the parsed JSON body.
func apiRequest(method, path string, payload any) (map[string]any, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, err
	}
	if c.Server == "" {
		return nil, errors.New("no server configured — run: boiler login --server <url>")
	}
	if err := c.ensureToken(); err != nil {
		return nil, err
	}
	do := func() (*http.Response, error) {
		var rdr io.Reader
		if payload != nil {
			b, _ := json.Marshal(payload)
			rdr = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, c.Server+path, rdr)
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return httpClient.Do(req)
	}
	res, err := do()
	if err != nil {
		return nil, err
	}
	if res.StatusCode == 401 {
		res.Body.Close()
		c.ExpiresAt = 0
		if e := c.ensureToken(); e != nil {
			return nil, e
		}
		if res, err = do(); err != nil {
			return nil, err
		}
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var body map[string]any
	if len(raw) > 0 {
		json.Unmarshal(raw, &body)
	}
	if res.StatusCode >= 400 {
		msg := ""
		if body != nil {
			if e, ok := body["error"].(string); ok {
				msg = e
			}
		}
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		return body, fmt.Errorf("%s (%d)", msg, res.StatusCode)
	}
	return body, nil
}

// apiRequestArray is for endpoints that return a top-level JSON array.
func apiRequestRaw(method, path string, payload any) ([]byte, int, error) {
	c, err := loadConfig()
	if err != nil {
		return nil, 0, err
	}
	if err := c.ensureToken(); err != nil {
		return nil, 0, err
	}
	var rdr io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.Server+path, rdr)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	return raw, res.StatusCode, nil
}

// --- output ---

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

// --- critical-action gate ---
// Critical/destructive commands require --confirm. The human still approves the
// Bash command in Claude Code; --confirm forces the agent to be explicit and makes
// the intent visible in the approved command.
var confirmFlag bool

func mustConfirm(summary string) error {
	if confirmFlag {
		return nil
	}
	return fmt.Errorf("⚠ critical action: %s\n  This is destructive/sensitive. Re-run with --confirm to execute.", summary)
}

// helper: get database flag or error
func reqStr(cmd *cobra.Command, name string) (string, error) {
	v, _ := cmd.Flags().GetString(name)
	if v == "" {
		return "", fmt.Errorf("--%s is required", name)
	}
	return v, nil
}
