package main

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"sync"
)

var templates *template.Template

type AppState struct {
	mu       sync.RWMutex
	mode     string // "test" or "prod"
	clients  map[string]*RazorpayClient
}

func NewAppState() *AppState {
	return &AppState{
		clients: make(map[string]*RazorpayClient),
	}
}

func (s *AppState) Client() *RazorpayClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.clients[s.mode]
}

func (s *AppState) Mode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

func (s *AppState) SetMode(mode string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.clients[mode]; !ok {
		return false
	}
	s.mode = mode
	return true
}

func loadKeysFromCSV(path string) (keyID, keySecret string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		return "", "", err
	}
	if len(records) < 2 {
		return "", "", fmt.Errorf("csv must have a header row and a data row")
	}
	row := records[1]
	if len(row) < 2 {
		return "", "", fmt.Errorf("csv data row must have key_id and key_secret columns")
	}
	return row[0], row[1], nil
}

func main() {
	webhookSecret := os.Getenv("RZP_WEBHOOK_SECRET")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	state := NewAppState()

	for _, mode := range []string{"test", "prod"} {
		csvFile := fmt.Sprintf("rzp-key-%s.csv", mode)
		if id, secret, err := loadKeysFromCSV(csvFile); err == nil {
			state.clients[mode] = NewRazorpayClient(id, secret)
			fmt.Printf("Loaded %s credentials from %s (key: %s)\n", mode, csvFile, id)
		} else {
			fmt.Printf("WARNING: could not load %s: %v\n", csvFile, err)
		}
	}

	// Default to test mode
	if !state.SetMode("test") {
		if !state.SetMode("prod") {
			log.Fatal("No credentials loaded. Place rzp-key-test.csv and/or rzp-key-prod.csv in the working directory.")
		}
	}

	var err error
	templates, err = template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// Routes
	mux := http.NewServeMux()

	// UI
	mux.HandleFunc("/", handleIndex(state))

	// Mode switch
	mux.HandleFunc("/api/mode", handleMode(state))

	// Orders
	mux.HandleFunc("/api/orders/create", handleCreateOrder(state))
	mux.HandleFunc("/api/orders/fetch", handleFetchOrder(state))
	mux.HandleFunc("/api/orders/payments", handleFetchOrderPayments(state))

	// Payments
	mux.HandleFunc("/api/payments/fetch", handleFetchPayment(state))
	mux.HandleFunc("/api/payments/capture", handleCapturePayment(state))
	mux.HandleFunc("/api/payments/refund", handleRefundPayment(state))

	// QR Codes
	mux.HandleFunc("/api/qr/create", handleCreateQR(state))
	mux.HandleFunc("/api/qr/fetch", handleFetchQR(state))
	mux.HandleFunc("/api/qr/close", handleCloseQR(state))
	mux.HandleFunc("/api/qr/list", handleListQR(state))

	// Webhooks
	mux.HandleFunc("/webhook", handleWebhook(webhookSecret))

	fmt.Printf("Razorpay Test App starting on http://localhost:%s\n", port)
	fmt.Printf("Active mode: %s (Key: %s)\n", state.Mode(), state.Client().KeyID)
	fmt.Printf("Webhook endpoint: http://localhost:%s/webhook\n", port)

	log.Fatal(http.ListenAndServe(":"+port, mux))
}
