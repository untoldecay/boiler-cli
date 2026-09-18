package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func authCommands() []*cobra.Command {
	login := &cobra.Command{
		Use:   "login",
		Short: "Log in to a Boiler instance (stores a short-lived token)",
		RunE: func(cmd *cobra.Command, args []string) error {
			server, _ := cmd.Flags().GetString("server")
			email, _ := cmd.Flags().GetString("email")
			password, _ := cmd.Flags().GetString("password")
			if server == "" {
				return fmt.Errorf("--server is required, e.g. --server https://boiler.example.com")
			}
			server = strings.TrimRight(server, "/")
			r := bufio.NewReader(os.Stdin)
			if email == "" {
				fmt.Print("Email: ")
				line, _ := r.ReadString('\n')
				email = strings.TrimSpace(line)
			}
			if password == "" {
				fmt.Print("Password: ")
				b, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Println()
				if err != nil {
					return err
				}
				password = string(b)
			}
			body, _ := json.Marshal(map[string]string{"email": email, "password": password})
			res, err := httpClient.Post(server+"/auth/login", "application/json", bytes.NewReader(body))
			if err != nil {
				return err
			}
			defer res.Body.Close()
			if res.StatusCode != 200 {
				return fmt.Errorf("login failed (%d)", res.StatusCode)
			}
			var out struct {
				AccessToken  string `json:"accessToken"`
				RefreshToken string `json:"refreshToken"`
				ExpiresIn    int    `json:"expiresIn"`
				User         struct {
					Email string `json:"email"`
					Role  string `json:"role"`
				} `json:"user"`
			}
			if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
				return err
			}
			if out.User.Role != "admin" {
				fmt.Printf("⚠ Logged in as %s (role: %s) — admin-only commands will be denied.\n", out.User.Email, out.User.Role)
			}
			cfg := &Config{
				Server: server, AccessToken: out.AccessToken, RefreshToken: out.RefreshToken,
				ExpiresAt: time.Now().Add(time.Duration(out.ExpiresIn) * time.Second).UnixMilli(),
				Email:     out.User.Email,
			}
			if err := saveConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("✓ Logged in to %s as %s (%s)\n", server, out.User.Email, out.User.Role)
			return nil
		},
	}
	login.Flags().String("server", "", "Boiler server URL")
	login.Flags().String("email", "", "email (prompted if omitted)")
	login.Flags().String("password", "", "password (prompted, hidden, if omitted)")

	logout := &cobra.Command{
		Use:   "logout",
		Short: "Forget the stored session",
		RunE: func(cmd *cobra.Command, args []string) error {
			os.Remove(configPath())
			fmt.Println("✓ Logged out")
			return nil
		},
	}

	whoami := &cobra.Command{
		Use:   "whoami",
		Short: "Show the current session",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := loadConfig()
			if err != nil {
				return err
			}
			fmt.Printf("Server: %s\nEmail:  %s\n", c.Server, c.Email)
			return nil
		},
	}

	status := &cobra.Command{
		Use:   "status",
		Short: "Show instance version + health",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := loadConfig()
			if err != nil {
				return err
			}
			res, err := http.Get(c.Server + "/api/version")
			if err != nil {
				return err
			}
			defer res.Body.Close()
			var v map[string]any
			json.NewDecoder(res.Body).Decode(&v)
			printJSON(v)
			return nil
		},
	}

	audit := &cobra.Command{
		Use:   "audit",
		Short: "Show recent admin/API actions (audit log)",
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			body, err := apiRequest("GET", fmt.Sprintf("/admin/audit?limit=%d", limit), nil)
			if err != nil {
				return err
			}
			printJSON(body["entries"])
			return nil
		},
	}
	audit.Flags().Int("limit", 50, "max entries")

	return []*cobra.Command{login, logout, whoami, status, audit}
}
