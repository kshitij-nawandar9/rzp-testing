package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const razorpayBaseURL = "https://api.razorpay.com/v1"

type RazorpayClient struct {
	KeyID     string
	KeySecret string
	HTTP      *http.Client
}

func NewRazorpayClient(keyID, keySecret string) *RazorpayClient {
	return &RazorpayClient{
		KeyID:     keyID,
		KeySecret: keySecret,
		HTTP:      &http.Client{},
	}
}

func (c *RazorpayClient) do(method, path string, body interface{}) (map[string]interface{}, int, error) {
	var reqBody io.Reader
	var reqBytes []byte
	if body != nil {
		var err error
		reqBytes, err = json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(reqBytes)
	}

	url := razorpayBaseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(c.KeyID, c.KeySecret)
	req.Header.Set("Content-Type", "application/json")

	fmt.Printf("[RZP] %s %s\n", method, url)
	if len(reqBytes) > 0 {
		fmt.Printf("[RZP] Request: %s\n", string(reqBytes))
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	fmt.Printf("[RZP] Response (%d): %s\n", resp.StatusCode, string(respBody))

	var result map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &result); err != nil {
			return nil, resp.StatusCode, fmt.Errorf("unmarshal response: %s", string(respBody))
		}
	}

	return result, resp.StatusCode, nil
}

// Orders

func (c *RazorpayClient) CreateOrder(amount int, currency, receipt string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"amount":   amount,
		"currency": currency,
		"receipt":  receipt,
	}
	result, status, err := c.do("POST", "/orders", body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}

func (c *RazorpayClient) FetchOrder(orderID string) (map[string]interface{}, error) {
	result, status, err := c.do("GET", "/orders/"+orderID, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}

func (c *RazorpayClient) FetchOrderPayments(orderID string) (map[string]interface{}, error) {
	result, status, err := c.do("GET", "/orders/"+orderID+"/payments", nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}

// Payments

func (c *RazorpayClient) FetchPayment(paymentID string) (map[string]interface{}, error) {
	result, status, err := c.do("GET", "/payments/"+paymentID, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}

func (c *RazorpayClient) CapturePayment(paymentID string, amount int, currency string) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"amount":   amount,
		"currency": currency,
	}
	result, status, err := c.do("POST", "/payments/"+paymentID+"/capture", body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}

func (c *RazorpayClient) RefundPayment(paymentID string, amount int) (map[string]interface{}, error) {
	body := map[string]interface{}{
		"amount": amount,
	}
	result, status, err := c.do("POST", "/payments/"+paymentID+"/refund", body)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}

// QR Codes

func (c *RazorpayClient) CreateQRCode(params map[string]interface{}) (map[string]interface{}, error) {
	result, status, err := c.do("POST", "/payments/qr_codes", params)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}

func (c *RazorpayClient) FetchQRCode(qrID string) (map[string]interface{}, error) {
	result, status, err := c.do("GET", "/payments/qr_codes/"+qrID, nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}

func (c *RazorpayClient) CloseQRCode(qrID string) (map[string]interface{}, error) {
	result, status, err := c.do("POST", "/payments/qr_codes/"+qrID+"/close", nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}

func (c *RazorpayClient) ListQRCodes() (map[string]interface{}, error) {
	result, status, err := c.do("GET", "/payments/qr_codes?count=10", nil)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("API error (%d): %v", status, result)
	}
	return result, nil
}
