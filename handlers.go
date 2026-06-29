package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type WebhookEntry struct {
	Timestamp string
	Event     string
	Body      string
}

var (
	webhookLog []WebhookEntry
	webhookMu  sync.Mutex
)

func handleIndex(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		client := state.Client()
		webhookMu.Lock()
		wh := make([]WebhookEntry, len(webhookLog))
		copy(wh, webhookLog)
		webhookMu.Unlock()

		modes := []string{}
		for m := range state.clients {
			modes = append(modes, m)
		}

		data := map[string]interface{}{
			"Connected": client.KeyID != "" && client.KeySecret != "",
			"KeyID":     client.KeyID,
			"Mode":      state.Mode(),
			"Modes":     modes,
			"Webhooks":  wh,
		}
		if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
			http.Error(w, err.Error(), 500)
		}
	}
}

func handleMode(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			jsonResponse(w, map[string]interface{}{
				"mode":   state.Mode(),
				"key_id": state.Client().KeyID,
			})
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		mode := r.FormValue("mode")
		if !state.SetMode(mode) {
			jsonError(w, fmt.Sprintf("unknown mode %q — no credentials loaded for it", mode), 400)
			return
		}
		client := state.Client()
		fmt.Printf("[MODE] Switched to %s (key: %s)\n", mode, client.KeyID)
		jsonResponse(w, map[string]interface{}{
			"mode":   mode,
			"key_id": client.KeyID,
		})
	}
}

// --- Orders ---

func handleCreateOrder(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		amountStr := r.FormValue("amount")
		currency := r.FormValue("currency")
		receipt := r.FormValue("receipt")

		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			jsonError(w, "Invalid amount: "+amountStr, 400)
			return
		}

		result, err := state.Client().CreateOrder(amount, currency, receipt)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

func handleFetchOrder(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderID := r.URL.Query().Get("order_id")
		if orderID == "" {
			jsonError(w, "order_id is required", 400)
			return
		}
		result, err := state.Client().FetchOrder(orderID)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

func handleFetchOrderPayments(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderID := r.URL.Query().Get("order_id")
		if orderID == "" {
			jsonError(w, "order_id is required", 400)
			return
		}
		result, err := state.Client().FetchOrderPayments(orderID)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

// --- Payments ---

func handleFetchPayment(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		paymentID := r.URL.Query().Get("payment_id")
		if paymentID == "" {
			jsonError(w, "payment_id is required", 400)
			return
		}
		result, err := state.Client().FetchPayment(paymentID)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

func handleCapturePayment(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		paymentID := r.FormValue("payment_id")
		amountStr := r.FormValue("amount")
		currency := r.FormValue("currency")

		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			jsonError(w, "Invalid amount", 400)
			return
		}

		result, err := state.Client().CapturePayment(paymentID, amount, currency)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

func handleRefundPayment(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		paymentID := r.FormValue("payment_id")
		amountStr := r.FormValue("amount")

		amount, err := strconv.Atoi(amountStr)
		if err != nil {
			jsonError(w, "Invalid amount", 400)
			return
		}

		result, err := state.Client().RefundPayment(paymentID, amount)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

// --- QR Codes ---

func handleCreateQR(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}

		qrType := r.FormValue("type")
		usage := r.FormValue("usage")
		description := r.FormValue("description")
		customerName := r.FormValue("customer_name")
		customerEmail := r.FormValue("customer_email")
		customerContact := r.FormValue("customer_contact")

		params := map[string]interface{}{
			"type":           qrType,
			"usage":          usage,
			"description":    description,
			"fixed_amount":   true,
			"payment_amount": 100,
		}

		if amountStr := r.FormValue("payment_amount"); amountStr != "" {
			amount, err := strconv.Atoi(amountStr)
			if err == nil {
				params["payment_amount"] = amount
				params["fixed_amount"] = true
			}
		}

		if usage == "single_use" {
			params["close_by"] = time.Now().Add(30 * time.Minute).Unix()
		}

		if customerName != "" || customerEmail != "" || customerContact != "" {
			customer := map[string]string{}
			if customerName != "" {
				customer["name"] = customerName
			}
			if customerEmail != "" {
				customer["email"] = customerEmail
			}
			if customerContact != "" {
				customer["contact"] = customerContact
			}
			params["customer"] = customer
		}

		result, err := state.Client().CreateQRCode(params)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

func handleFetchQR(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		qrID := r.URL.Query().Get("qr_id")
		if qrID == "" {
			jsonError(w, "qr_id is required", 400)
			return
		}
		result, err := state.Client().FetchQRCode(qrID)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

func handleCloseQR(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}
		qrID := r.FormValue("qr_id")
		if qrID == "" {
			jsonError(w, "qr_id is required", 400)
			return
		}
		result, err := state.Client().CloseQRCode(qrID)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

func handleListQR(state *AppState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := state.Client().ListQRCodes()
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		jsonResponse(w, result)
	}
}

// --- Webhooks ---

func handleWebhook(webhookSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", 400)
			return
		}

		// Verify signature if secret is set
		if webhookSecret != "" {
			sig := r.Header.Get("X-Razorpay-Signature")
			if sig != "" {
				mac := hmac.New(sha256.New, []byte(webhookSecret))
				mac.Write(body)
				expected := hex.EncodeToString(mac.Sum(nil))
				if !hmac.Equal([]byte(expected), []byte(sig)) {
					fmt.Printf("[WEBHOOK] Signature verification FAILED\n")
				} else {
					fmt.Printf("[WEBHOOK] Signature verified OK\n")
				}
			}
		}

		var payload map[string]interface{}
		event := "unknown"
		if err := json.Unmarshal(body, &payload); err == nil {
			if e, ok := payload["event"].(string); ok {
				event = e
			}
		}

		prettyBody, _ := json.MarshalIndent(payload, "", "  ")

		entry := WebhookEntry{
			Timestamp: time.Now().Format("2006-01-02 15:04:05"),
			Event:     event,
			Body:      string(prettyBody),
		}

		webhookMu.Lock()
		webhookLog = append([]WebhookEntry{entry}, webhookLog...) // prepend (newest first)
		if len(webhookLog) > 50 {
			webhookLog = webhookLog[:50]
		}
		webhookMu.Unlock()

		fmt.Printf("[WEBHOOK] %s - %s\n", entry.Timestamp, event)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
}

// --- Helpers ---

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
