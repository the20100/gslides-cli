package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/the20100/g-slides-cli/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage Google Slides API authentication",
}

var (
	serviceAccountFile string
	credentialsFile    string
	clientIDFlag       string
	clientSecretFlag   string
)

var authSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure authentication (service account or OAuth2)",
	Long: `Configure authentication for the Google Slides API.

─── Option A: Service Account (recommended for automation) ─────────────────
  1. Create a service account in Google Cloud Console
  2. Enable the Google Slides API for your project
  3. Download the JSON key file

  gslides auth setup --service-account /path/to/sa.json
  # or set: export GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa.json

─── Option B: OAuth2 (for manual / interactive use) ────────────────────────
  1. Create OAuth2 credentials (Desktop app) at:
     https://console.cloud.google.com/apis/credentials
  2. Enable the Google Slides API for your project
  3. Add http://localhost:8080 as an authorized redirect URI

  gslides auth setup --credentials /path/to/credentials.json
  # or provide --client-id and --client-secret flags`,
	RunE: runAuthSetup,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runAuthStatus,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove saved credentials from the config file",
	RunE:  runAuthLogout,
}

func init() {
	authSetupCmd.Flags().StringVar(&serviceAccountFile, "service-account", "", "Path to a service account JSON key file")
	authSetupCmd.Flags().StringVar(&credentialsFile, "credentials", "", "Path to an OAuth2 credentials.json file (Desktop app)")
	authSetupCmd.Flags().StringVar(&clientIDFlag, "client-id", "", "OAuth2 client ID")
	authSetupCmd.Flags().StringVar(&clientSecretFlag, "client-secret", "", "OAuth2 client secret")

	authCmd.AddCommand(authSetupCmd, authStatusCmd, authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}

func runAuthSetup(cmd *cobra.Command, args []string) error {
	// ── Service account path ──────────────────────────────────────────────────
	if serviceAccountFile != "" {
		data, err := os.ReadFile(serviceAccountFile)
		if err != nil {
			return fmt.Errorf("reading service account file: %w", err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("invalid JSON in service account file: %w", err)
		}
		if raw["type"] != "service_account" {
			return fmt.Errorf("file does not appear to be a service account key (type=%q)", raw["type"])
		}

		cfg := &config.Config{
			AuthMethod:         config.AuthMethodServiceAccount,
			ServiceAccountJSON: string(data),
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		saEmail, _ := raw["client_email"].(string)
		fmt.Printf("Service account configured: %s\n", saEmail)
		fmt.Printf("Config saved to: %s\n", config.Path())
		return nil
	}

	// ── OAuth2 path ───────────────────────────────────────────────────────────
	clientID := clientIDFlag
	clientSecret := clientSecretFlag

	if credentialsFile != "" {
		data, err := os.ReadFile(credentialsFile)
		if err != nil {
			return fmt.Errorf("reading credentials file: %w", err)
		}
		creds, err := parseCredentialsJSON(data)
		if err != nil {
			return fmt.Errorf("parsing credentials.json: %w", err)
		}
		clientID = creds.ClientID
		clientSecret = creds.ClientSecret
	}

	if clientID == "" {
		clientID = os.Getenv("GOOGLE_CLIENT_ID")
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		if existing, _ := config.Load(); existing != nil {
			if clientID == "" {
				clientID = existing.ClientID
			}
			if clientSecret == "" {
				clientSecret = existing.ClientSecret
			}
		}
	}

	if clientID == "" {
		fmt.Print("Enter OAuth2 Client ID: ")
		fmt.Scan(&clientID)
	}
	if clientSecret == "" {
		fmt.Print("Enter OAuth2 Client Secret: ")
		fmt.Scan(&clientSecret)
	}

	clientID = strings.TrimSpace(clientID)
	clientSecret = strings.TrimSpace(clientSecret)

	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("client ID and secret are required for OAuth2 setup")
	}

	oauthCfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  "http://localhost:8080",
		Scopes: []string{
			"https://www.googleapis.com/auth/presentations",
			"https://www.googleapis.com/auth/drive.file",
		},
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	server := &http.Server{Addr: ":8080", Handler: mux}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			if errMsg == "" {
				errMsg = "unknown error"
			}
			errCh <- fmt.Errorf("authorization failed: %s", errMsg)
			fmt.Fprintf(w, "<html><body><h2>Authorization failed: %s</h2><p>You can close this tab.</p></body></html>", errMsg)
			return
		}
		codeCh <- code
		fmt.Fprintf(w, "<html><body><h2>Authorization successful!</h2><p>You can close this tab and return to the terminal.</p></body></html>")
	})

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("local callback server error: %w", err)
		}
	}()

	authURL := oauthCfg.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Printf("\nOpening browser for authorization...\n")
	fmt.Printf("If the browser doesn't open, visit:\n\n  %s\n\n", authURL)
	openBrowser(authURL)

	fmt.Println("Waiting for authorization (timeout: 5 minutes)...")

	var code string
	select {
	case code = <-codeCh:
	case err := <-errCh:
		server.Shutdown(context.Background())
		return err
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		return fmt.Errorf("authorization timed out")
	}

	server.Shutdown(context.Background())

	token, err := oauthCfg.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("exchanging code for token: %w", err)
	}

	cfg := &config.Config{
		AuthMethod:   config.AuthMethodOAuth2,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		TokenExpiry:  token.Expiry,
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\nAuthentication successful! Config saved to:\n  %s\n", config.Path())
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Printf("Config: %s\n\n", config.Path())

	if sa := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); sa != "" {
		fmt.Printf("Auth method: service account (GOOGLE_APPLICATION_CREDENTIALS)\n")
		fmt.Printf("File:        %s\n", sa)
		return nil
	}

	switch cfg.AuthMethod {
	case config.AuthMethodServiceAccount:
		fmt.Println("Auth method: service account")
		var raw map[string]any
		if err := json.Unmarshal([]byte(cfg.ServiceAccountJSON), &raw); err == nil {
			fmt.Printf("Account:     %s\n", raw["client_email"])
			fmt.Printf("Project:     %s\n", raw["project_id"])
		}
	case config.AuthMethodOAuth2:
		fmt.Println("Auth method: OAuth2")
		if os.Getenv("GOOGLE_CLIENT_ID") != "" {
			fmt.Println("Client ID:   (from GOOGLE_CLIENT_ID env var)")
		} else {
			fmt.Printf("Client ID:   %s\n", maskString(cfg.ClientID))
		}
		if cfg.RefreshToken != "" || cfg.AccessToken != "" {
			fmt.Println("Status:      authenticated")
			if !cfg.TokenExpiry.IsZero() {
				if cfg.TokenExpiry.Before(time.Now()) {
					fmt.Println("Token:       expired (will refresh automatically)")
				} else {
					fmt.Printf("Token:       valid until %s\n", cfg.TokenExpiry.Format(time.RFC3339))
				}
			}
		} else {
			fmt.Println("Status:      not authenticated — run: gslides auth setup")
		}
	default:
		fmt.Println("Auth method: not configured")
		fmt.Println("\nRun: gslides auth setup --service-account sa.json")
		fmt.Println("Or:  gslides auth setup --credentials credentials.json")
	}
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	if err := config.Clear(); err != nil {
		return fmt.Errorf("removing config: %w", err)
	}
	fmt.Println("Credentials removed from config.")
	return nil
}

// credentialsJSONEntry holds the client credentials within a credentials.json file.
type credentialsJSONEntry struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type credentialsJSONFile struct {
	Installed *credentialsJSONEntry `json:"installed"`
	Web       *credentialsJSONEntry `json:"web"`
}

func parseCredentialsJSON(data []byte) (*credentialsJSONEntry, error) {
	var wrapper credentialsJSONFile
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	if wrapper.Installed != nil {
		return wrapper.Installed, nil
	}
	if wrapper.Web != nil {
		return wrapper.Web, nil
	}
	return nil, fmt.Errorf("could not find 'installed' or 'web' credentials in file")
}
