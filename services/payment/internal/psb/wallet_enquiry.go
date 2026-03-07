package psb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const walletEnquiryPath = "/api/v1/wallet_enquiry"

// WalletEnquiryRequest is the request body for wallet_enquiry.
type WalletEnquiryRequest struct {
	AccountNo string `json:"accountNo"`
}

// WalletEnquiryData is the successful response data from 9PSB.
type WalletEnquiryData struct {
	ProductCode         string  `json:"productCode"`
	ResponseCode        string  `json:"responseCode"`
	PhoneNo             string  `json:"phoneNo"`
	Tier                string  `json:"tier"`
	IsSuccessful        bool    `json:"isSuccessful"`
	LedgerBalance       float64 `json:"ledgerBalance"`
	AvailableBalance    float64 `json:"availableBalance"`
	Bvn                 string  `json:"bvn"`
	Number              string  `json:"number"`
	Nuban               string  `json:"nuban"`
	MaximumBalance      float64 `json:"maximumBalance"`
	ResponseDescription string  `json:"responseDescription"`
	PhoneNuber          string  `json:"phoneNuber"`
	Pndstatus           string  `json:"pndstatus"`
	LienStatus          string  `json:"lienStatus"`
	FreezeStatus        string  `json:"freezeStatus"`
	Status              string  `json:"status"`
	Name                string  `json:"name"`
}

// WalletEnquiryResponse is the full 9PSB wallet_enquiry response.
type WalletEnquiryResponse struct {
	Status       string              `json:"status"`
	ResponseCode string              `json:"responseCode"`
	Message      string              `json:"message"`
	Data         *WalletEnquiryData   `json:"data"`
}

// WalletEnquiryResult is the parsed result returned to callers (live balance from 9PSB).
type WalletEnquiryResult struct {
	AvailableBalance float64
	LedgerBalance    float64
	Nuban            string
	Name             string
	Status           string
	IsSuccessful     bool
}

// WalletEnquiry calls 9PSB wallet_enquiry for the given account number. Returns live balance from provider.
func (p *TokenProvider) WalletEnquiry(ctx context.Context, accountNo string) (*WalletEnquiryResult, error) {
	token, err := p.GetToken(ctx)
	if err != nil {
		return nil, err
	}
	body := WalletEnquiryRequest{AccountNo: accountNo}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+walletEnquiryPath, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out WalletEnquiryResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("9PSB wallet_enquiry: invalid JSON: %w", err)
	}
	if out.Status != "SUCCESS" || out.Data == nil {
		return nil, fmt.Errorf("9PSB wallet_enquiry: %s (status=%s)", out.Message, out.Status)
	}
	if !out.Data.IsSuccessful {
		return nil, fmt.Errorf("9PSB wallet_enquiry: %s", out.Message)
	}
	return &WalletEnquiryResult{
		AvailableBalance: out.Data.AvailableBalance,
		LedgerBalance:    out.Data.LedgerBalance,
		Nuban:            out.Data.Nuban,
		Name:             out.Data.Name,
		Status:           out.Data.Status,
		IsSuccessful:     out.Data.IsSuccessful,
	}, nil
}
