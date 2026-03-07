package psb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	waasDebitPath  = "/api/v1/debit/transfer"
	waasCreditPath = "/api/v1/credit/transfer"
)

// WaasTransferRequest is the request body for 9PSB WaaS debit/credit.
type WaasTransferRequest struct {
	AccountNo     string  `json:"accountNo"`
	Narration     string  `json:"narration"`
	TotalAmount   float64 `json:"totalAmount"`
	TransactionID string  `json:"transactionId"`
	Merchant      struct {
		IsFee bool `json:"isFee"`
	} `json:"merchant"`
}

// WaasTransferResponse is the 9PSB WaaS debit/credit response.
type WaasTransferResponse struct {
	Message string `json:"message"`
	Status  string `json:"status"` // "success" or "FAILED"
	Data    struct {
		ResponseCode string  `json:"responseCode"`
		Reference    *string `json:"reference"`
	} `json:"data"`
}

// WaasDebitTransfer calls 9PSB WaaS debit endpoint. Returns (reference, nil) on success; error on failure or duplicate.
func (p *TokenProvider) WaasDebitTransfer(ctx context.Context, accountNo, narration string, totalAmount float64, transactionID string) (reference string, err error) {
	return p.waasTransfer(ctx, waasDebitPath, accountNo, narration, totalAmount, transactionID)
}

// WaasCreditTransfer calls 9PSB WaaS credit endpoint. Returns (reference, nil) on success; error on failure or duplicate.
func (p *TokenProvider) WaasCreditTransfer(ctx context.Context, accountNo, narration string, totalAmount float64, transactionID string) (reference string, err error) {
	return p.waasTransfer(ctx, waasCreditPath, accountNo, narration, totalAmount, transactionID)
}

func (p *TokenProvider) waasTransfer(ctx context.Context, path, accountNo, narration string, totalAmount float64, transactionID string) (reference string, err error) {
	if p.waasBaseURL == "" {
		return "", fmt.Errorf("9PSB WaaS base URL not configured")
	}
	token, err := p.GetToken(ctx)
	if err != nil {
		return "", fmt.Errorf("9PSB auth: %w", err)
	}
	body := WaasTransferRequest{
		AccountNo:     accountNo,
		Narration:     narration,
		TotalAmount:   totalAmount,
		TransactionID: transactionID,
	}
	body.Merchant.IsFee = false
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	url := p.waasBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := p.httpClient
	if p.waasClient != nil {
		client = p.waasClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("9PSB WaaS request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("9PSB WaaS response (HTTP %d): %w", resp.StatusCode, err)
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("9PSB WaaS response (HTTP %d): empty body", resp.StatusCode)
	}
	var out WaasTransferResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("9PSB WaaS invalid JSON (HTTP %d): %w", resp.StatusCode, err)
	}
	if out.Status == "success" && out.Data.ResponseCode == "00" {
		if out.Data.Reference != nil {
			return *out.Data.Reference, nil
		}
		return "", nil
	}
	msg := out.Message
	if msg == "" {
		msg = fmt.Sprintf("responseCode=%s", out.Data.ResponseCode)
	}
	return "", fmt.Errorf("9PSB WaaS failed: %s", msg)
}

// waasPost sends a POST request to the WaaS API and returns the response body and status code. Uses waasClient when set (custom transport).
func (p *TokenProvider) waasPost(ctx context.Context, path string, reqBody interface{}) ([]byte, int, error) {
	if p.waasBaseURL == "" {
		return nil, 0, fmt.Errorf("9PSB WaaS base URL not configured")
	}
	token, err := p.GetToken(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("9PSB auth: %w", err)
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, err
	}
	url := p.waasBaseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := p.httpClient
	if p.waasClient != nil {
		client = p.waasClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("9PSB WaaS request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("9PSB WaaS response: %w", err)
	}
	return raw, resp.StatusCode, nil
}
