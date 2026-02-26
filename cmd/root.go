package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/the20100/g-slides-cli/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	slides "google.golang.org/api/slides/v1"
)

var (
	jsonFlag   bool
	prettyFlag bool

	// Global Slides API service, initialised in PersistentPreRunE.
	svc *slides.Service
)

var rootCmd = &cobra.Command{
	Use:   "gslides",
	Short: "Google Slides CLI — manage presentations via the API",
	Long: `gslides is a CLI tool for the Google Slides API v1.

Create and modify Google Slides presentations programmatically.

Auth methods supported:
  1. Service account (recommended for automation):
       gslides auth setup --service-account /path/to/sa.json
     Or set: GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa.json

  2. OAuth2 (for interactive / manual use):
       gslides auth setup --credentials /path/to/credentials.json

Token resolution order:
  1. GOOGLE_APPLICATION_CREDENTIALS env var (service account)
  2. Config file (~/.config/g-slides/config.json via: gslides auth setup)

Examples:
  gslides auth setup --service-account sa.json
  gslides presentation create "My Deck"
  gslides presentation get <presentation-id>
  gslides presentation slides <presentation-id>
  gslides slide add <presentation-id>
  gslides slide thumbnail <presentation-id> <slide-id>`,
	SilenceUsage: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Force JSON output")
	rootCmd.PersistentFlags().BoolVar(&prettyFlag, "pretty", false, "Force pretty-printed JSON output (implies --json)")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if isAuthCommand(cmd) || cmd.Name() == "info" || cmd.Name() == "update" {
			return nil
		}
		return initService(cmd.Context())
	}

	rootCmd.AddCommand(infoCmd)
}

// savingTokenSource wraps an oauth2.TokenSource and persists refreshed tokens to config.
type savingTokenSource struct {
	source oauth2.TokenSource
	cfg    *config.Config
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	token, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	if token.AccessToken != s.cfg.AccessToken {
		s.cfg.AccessToken = token.AccessToken
		s.cfg.TokenExpiry = token.Expiry
		_ = config.Save(s.cfg)
	}
	return token, nil
}

func initService(ctx context.Context) error {
	// 1. GOOGLE_APPLICATION_CREDENTIALS env var — service account.
	if sa := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); sa != "" {
		data, err := os.ReadFile(sa)
		if err != nil {
			return fmt.Errorf("reading GOOGLE_APPLICATION_CREDENTIALS file: %w", err)
		}
		svc, err = slides.NewService(ctx, option.WithCredentialsJSON(data))
		if err != nil {
			return fmt.Errorf("creating service with service account: %w", err)
		}
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	switch cfg.AuthMethod {

	// 2. Service account stored in config.
	case config.AuthMethodServiceAccount:
		if cfg.ServiceAccountJSON == "" {
			return fmt.Errorf("no service account JSON in config — run: gslides auth setup")
		}
		svc, err = slides.NewService(ctx, option.WithCredentialsJSON([]byte(cfg.ServiceAccountJSON)))
		if err != nil {
			return fmt.Errorf("creating service with service account: %w", err)
		}
		return nil

	// 3. OAuth2 stored in config.
	case config.AuthMethodOAuth2:
		clientID := os.Getenv("GOOGLE_CLIENT_ID")
		if clientID == "" {
			clientID = cfg.ClientID
		}
		clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
		if clientSecret == "" {
			clientSecret = cfg.ClientSecret
		}
		if clientID == "" || clientSecret == "" {
			return fmt.Errorf("OAuth2 client ID/secret missing — run: gslides auth setup")
		}
		if cfg.RefreshToken == "" && cfg.AccessToken == "" {
			return fmt.Errorf("not authenticated — run: gslides auth setup")
		}

		oauthCfg := &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     google.Endpoint,
			Scopes: []string{
				"https://www.googleapis.com/auth/presentations",
				"https://www.googleapis.com/auth/drive.file",
			},
		}
		token := &oauth2.Token{
			AccessToken:  cfg.AccessToken,
			RefreshToken: cfg.RefreshToken,
			TokenType:    cfg.TokenType,
			Expiry:       cfg.TokenExpiry,
		}
		ts := oauthCfg.TokenSource(ctx, token)
		savingTS := &savingTokenSource{source: ts, cfg: cfg}
		httpClient := oauth2.NewClient(ctx, savingTS)

		svc, err = slides.NewService(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			return fmt.Errorf("creating service with OAuth2: %w", err)
		}
		return nil

	default:
		return fmt.Errorf("not configured — run: gslides auth setup\nor set GOOGLE_APPLICATION_CREDENTIALS env var")
	}
}

// isAuthCommand returns true if cmd is the "auth" command or one of its children.
func isAuthCommand(cmd *cobra.Command) bool {
	if cmd.Name() == "auth" {
		return true
	}
	p := cmd.Parent()
	for p != nil {
		if p.Name() == "auth" {
			return true
		}
		p = p.Parent()
	}
	return false
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show tool info: config path, auth status, and environment",
	Run: func(cmd *cobra.Command, args []string) {
		printInfo()
	},
}

func printInfo() {
	fmt.Println("gslides — Google Slides CLI")
	fmt.Println()

	exe, _ := os.Executable()
	fmt.Printf("  binary:  %s\n", exe)
	fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	fmt.Println("  config paths by OS:")
	fmt.Println("    macOS:    ~/Library/Application Support/g-slides/config.json")
	fmt.Println("    Linux:    ~/.config/g-slides/config.json")
	fmt.Println("    Windows:  %AppData%\\g-slides\\config.json")
	fmt.Printf("  config:   %s\n", config.Path())
	fmt.Println()

	cfg, _ := config.Load()

	if sa := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); sa != "" {
		fmt.Printf("  auth method: service account (GOOGLE_APPLICATION_CREDENTIALS=%s)\n", sa)
	} else if cfg != nil && cfg.AuthMethod == config.AuthMethodServiceAccount {
		fmt.Println("  auth method: service account (config file)")
	} else if cfg != nil && cfg.AuthMethod == config.AuthMethodOAuth2 {
		authStatus := "authenticated"
		if cfg.TokenExpiry.IsZero() || cfg.TokenExpiry.Before(time.Now()) {
			authStatus += " (access token expired, will refresh)"
		}
		fmt.Printf("  auth method: OAuth2 — %s\n", authStatus)
	} else {
		fmt.Println("  auth method: not configured")
	}

	fmt.Println()
	fmt.Println("  credential resolution order:")
	fmt.Println("    1. GOOGLE_APPLICATION_CREDENTIALS env var (service account)")
	fmt.Println("    2. config file — service account or OAuth2  (gslides auth setup)")
}
