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
	otherBanksEnquiryPath  = "/api/v1/other_banks_enquiry"
	walletOtherBanksPath   = "/api/v1/wallet_other_banks"
)

// OtherBanksEnquiryRequest is the request for account name enquiry.
type OtherBanksEnquiryRequest struct {
	Customer struct {
		Account struct {
			Bank   string `json:"bank"`
			Number string `json:"number"`
		} `json:"account"`
	} `json:"customer"`
}

// OtherBanksEnquiryData is the successful response data (legacy status/data shape).
type OtherBanksEnquiryData struct {
	Name   string `json:"name"`
	Number string `json:"number"`
	Bank   string `json:"bank"`
}

// OtherBanksEnquiryResponseLegacy is the legacy response shape (status + data).
type OtherBanksEnquiryResponseLegacy struct {
	Status string                 `json:"status"`
	Data   *OtherBanksEnquiryData `json:"data"`
}

// OtherBanksEnquiryResponse is the actual 9PSB response (code + customer.account).
type OtherBanksEnquiryResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Customer struct {
		Account struct {
			Number string `json:"number"`
			Bank   string `json:"bank"`
			Name   string `json:"name"`
		} `json:"account"`
	} `json:"customer"`
}

// OtherBanksEnquiry calls 9PSB other_banks_enquiry to resolve beneficiary name. Returns name or error.
// Supports both response shapes: code "00" with customer.account.name, or status "SUCCESS" with data.name.
func (p *TokenProvider) OtherBanksEnquiry(ctx context.Context, bankCode, accountNumber string) (beneficiaryName string, err error) {
	token, err := p.GetToken(ctx)
	if err != nil {
		return "", err
	}
	body := OtherBanksEnquiryRequest{}
	body.Customer.Account.Bank = bankCode
	body.Customer.Account.Number = accountNumber
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+otherBanksEnquiryPath, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// Try actual format first: code "00" and customer.account.name
	var out OtherBanksEnquiryResponse
	if err := json.Unmarshal(raw, &out); err == nil {
		if out.Code == "00" && out.Customer.Account.Name != "" {
			return out.Customer.Account.Name, nil
		}
		if out.Message != "" {
			return "", fmt.Errorf("9PSB other_banks_enquiry: %s (code=%s)", out.Message, out.Code)
		}
	}
	// Fallback: legacy status/data format
	var leg OtherBanksEnquiryResponseLegacy
	if err := json.Unmarshal(raw, &leg); err != nil {
		return "", fmt.Errorf("9PSB other_banks_enquiry: invalid JSON: %w", err)
	}
	if leg.Status == "SUCCESS" && leg.Data != nil && leg.Data.Name != "" {
		return leg.Data.Name, nil
	}
	return "", fmt.Errorf("9PSB other_banks_enquiry: account not found or invalid (code=%s)", out.Code)
}

// WalletOtherBanksPayload is the request body for wallet_other_banks (as per 9PSB spec).
type WalletOtherBanksPayload struct {
	Customer struct {
		Account struct {
			Bank                 string `json:"bank"`
			Name                 string `json:"name"`
			Number               string `json:"number"`
			SenderAccountNumber  string `json:"senderaccountnumber"`
			SenderName           string `json:"sendername"`
		} `json:"account"`
	} `json:"customer"`
	Narration string `json:"narration"`
	Order     struct {
		Amount      string `json:"amount"`
		Country     string `json:"country"`
		Currency    string `json:"currency"`
		Description string `json:"description"`
	} `json:"order"`
	Transaction struct {
		Reference string `json:"reference"`
	} `json:"transaction"`
	Merchant struct {
		IsFee              bool   `json:"isFee"`
		MerchantFeeAccount string `json:"merchantFeeAccount"`
		MerchantFeeAmount  string `json:"merchantFeeAmount"`
	} `json:"merchant"`
}

// WalletOtherBanksResponse is the response (body may be minimal; success = status SUCCESS or responseCode 00).
type WalletOtherBanksResponse struct {
	Status       string `json:"status"`
	ResponseCode string `json:"responseCode"`
	Message      string `json:"message"`
	Data         struct {
		SessionID string `json:"sessionID"`
		Amount    string `json:"amount"`
		Reference string `json:"reference"`
	} `json:"data"`
}

// WalletOtherBanks calls 9PSB wallet_other_banks (uses baseURL2 if set). Returns full response body and nil error on HTTP 200 + success.
func (p *TokenProvider) WalletOtherBanks(ctx context.Context, payload *WalletOtherBanksPayload) (rawResponse []byte, sessionID string, responseCode string, err error) {
	token, err := p.GetToken(ctx)
	if err != nil {
		return nil, "", "", err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", "", err
	}
	baseURL := p.baseURL2
	if baseURL == "" {
		baseURL = p.baseURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+walletOtherBanksPath, bytes.NewReader(body))
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", err
	}
	var out WalletOtherBanksResponse
	_ = json.Unmarshal(raw, &out)
	code := out.ResponseCode
	if code == "" {
		code = out.Status
	}
	// Success: responseCode "00" or status "SUCCESS"
	ok := code == "00" || out.Status == "SUCCESS"
	if !ok {
		return raw, out.Data.SessionID, code, fmt.Errorf("9PSB wallet_other_banks failed: %s (responseCode=%s)", out.Message, code)
	}
	return raw, out.Data.SessionID, code, nil
}
