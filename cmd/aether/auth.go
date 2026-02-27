package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/aethercore/aethercore/core"
)

// authCmd groups the login and onboard logic
func authCmd(mode string) {
	fmt.Printf("Starting AetherCore %s process...\n", mode)

	// In a real implementation this would fetch the well-known keys.
	// We'll initialize AuthManager with nil public key just for the CLI stub since
	// the actual PKI verification needs the RS256 public key from auth backend.
	_, err := core.NewAuthManager(nil)
	if err != nil {
		log.Fatalf("Failed to initialize auth manager: %v", err)
	}

	// Setup a local redirect server to receive the JWT from the cloud auth provider
	// Clerk.dev will redirect to localhost:9092/callback?token=xxx
	tokenChan := make(chan string)
	srv := &http.Server{
		Addr:              ":9092",
		ReadHeaderTimeout: 3 * time.Second,
	}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token parameter", http.StatusBadRequest)
			return
		}

		fmt.Fprintln(w, "Authentication successful! You can close this browser tab.")
		tokenChan <- token
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Local auth server failed: %v", err)
		}
	}()

	// Open the browser
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		log.Fatalf("Failed to generate random state: %v", err)
	}
	state := base64.URLEncoding.EncodeToString(stateBytes)

	// aethercore.dev is the placeholder cloud auth domain
	url := fmt.Sprintf("https://auth.aethercore.dev/%s?redirect_uri=http://localhost:9092/callback&state=%s", mode, state)
	fmt.Printf("Opening browser to %s\n", url)
	openBrowser(url)

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

	// Shutdown the server cleanly
	_ = srv.Shutdown(context.Background())
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		/* #nosec G204 */
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		/* #nosec G204 */
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		/* #nosec G204 */
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Printf("Failed to open browser automatically. Please visit: %s", url)
	}
}

// deleteCmd handles GDPR account deletion
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

	req, err := http.NewRequest("DELETE", "https://auth.aethercore.dev/account", nil)
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
