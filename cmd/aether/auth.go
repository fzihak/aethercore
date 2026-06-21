package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/fzihak/aethercore/core"
)

var ErrUnsupportedPlatform = errors.New("unsupported platform")

// loginCmd handles the authentication flow for existing users.
func loginCmd() {
	fmt.Printf("Starting AetherCore login process...\n")

	initAuthManager()

	// Open the browser
	state := generateState()

	srv, tokenChan := startAuthServer(state)

	// auth.aethercore.brainexia.com is the AetherCore cloud auth domain
	url := "https://auth.aethercore.brainexia.com/login?redirect_uri=http://localhost:9092/callback&state=" + state
	fmt.Printf("Opening browser to %s\n", url)
	openBrowser(url)

	waitForToken(tokenChan)

	// Shutdown the server cleanly
	_ = srv.Shutdown(context.Background())
}

// onboardCmd handles the first-time setup and authentication flow.
func onboardCmd() {
	fmt.Printf("Starting AetherCore signup process...\n")

	initAuthManager()

	// Open the browser
	state := generateState()

	srv, tokenChan := startAuthServer(state)

	// auth.aethercore.brainexia.com is the AetherCore cloud auth domain
	url := "https://auth.aethercore.brainexia.com/signup?redirect_uri=http://localhost:9092/callback&state=" + state
	fmt.Printf("Opening browser to %s\n", url)
	openBrowser(url)

	waitForToken(tokenChan)

	// Shutdown the server cleanly
	_ = srv.Shutdown(context.Background())
}

//nolint:gocritic,nonamedreturns // named return values are unnecessary for these two standard variables
func startAuthServer(expectedState string) (*http.Server, chan string) {
	// Setup a local redirect server to receive the JWT from the cloud auth provider
	// Clerk.dev will redirect to localhost:9092/callback?token=xxx
	tokenChan := make(chan string)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != expectedState {
			http.Error(w, "invalid state parameter", http.StatusBadRequest)
			return
		}

		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token parameter", http.StatusBadRequest)
			return
		}

		fmt.Fprintln(w, "Authentication successful! You can close this browser tab.")
		tokenChan <- token
	})

	srv := &http.Server{
		Addr:              "127.0.0.1:9092",
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Local auth server failed: %v", err)
		}
	}()

	return srv, tokenChan
}

func generateState() string {
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		log.Fatalf("Failed to generate random state: %v", err)
	}
	return base64.URLEncoding.EncodeToString(stateBytes)
}

func waitForToken(tokenChan <-chan string) {
	// Wait for the callback with a timeout
	select {
	case token := <-tokenChan:
		// We have the token! Save it to ~/.aether/token
		// We use a bypass to TokenStore because manager is private fields
		store, _ := core.NewTokenStore()
		if err := store.Save(token); err != nil {
			log.Fatalf("Failed to save token securely: %v", err)
		}
		fmt.Printf("Setup complete. AetherCore is fully operational in Pico Mode.\n")
		fmt.Printf("Try: aether run --goal 'Hello'\n")

	case <-time.After(5 * time.Minute):
		log.Fatalf("Authentication timed out after 5 minutes.")
	}
}

func initAuthManager() {
	// In a real implementation this would fetch the well-known keys.
	// We'll initialize AuthManager with nil public key just for the CLI stub since
	// the actual PKI verification needs the RS256 public key from auth backend.
	_, err := core.NewAuthManager(nil)
	if err != nil {
		log.Fatalf("Failed to initialize auth manager: %v", err)
	}
}

func openBrowser(targetURL string) {
	u, err := url.ParseRequestURI(targetURL)
	if err != nil {
		log.Printf("Failed to parse URL: %v. Please visit: %s", err, targetURL)
		return
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		log.Printf("Invalid URL scheme %q. Only http and https are allowed. Please visit: %s", u.Scheme, targetURL)
		return
	}

	safeURL := u.String()

	switch runtime.GOOS {
	case "linux":
		/* #nosec G204 */
		err = exec.Command("xdg-open", safeURL).Start()
	case "windows":
		/* #nosec G204 */
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", safeURL).Start()
	case "darwin":
		/* #nosec G204 */
		err = exec.Command("open", safeURL).Start()
	default:
		err = ErrUnsupportedPlatform
	}
	if err != nil {
		log.Printf("Failed to open browser automatically. Please visit: %s", safeURL)
	}
}

// deleteCmd handles GDPR account deletion.
//
//nolint:gocritic,mnd // CLI exits and local timeouts are intended
func deleteCmd() {
	fmt.Println("Deleting AetherCore account and associated cloud analytics...")

	store, err := core.NewTokenStore()
	if err != nil {
		log.Fatalf("Failed to initialize token store: %v", err)
	}

	token, err := store.Load()
	if err != nil {
		log.Fatalf("No active session found. Are you logged in?")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, "https://auth.aethercore.brainexia.com/account", nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to contact auth cloud: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Delete local token too
		_ = store.Delete()
		fmt.Println("Account deleted successfully. Local token removed.")
	} else {
		fmt.Printf("Failed to delete account. Status: %d\n", resp.StatusCode)
	}
}
