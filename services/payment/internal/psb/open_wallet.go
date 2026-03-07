package psb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

const openWalletPath = "/api/v1/open_wallet"

// OpenWalletRequest is the 9PSB open_wallet request body.
type OpenWalletRequest struct {
	BVN                    string `json:"bvn"`
	DateOfBirth            string `json:"dateOfBirth"`            // DD/MM/YYYY
	Gender                 int    `json:"gender"`                // 1 = Male, 2 = Female
	LastName               string `json:"lastName"`
	OtherNames             string `json:"otherNames"`
	PhoneNo                string `json:"phoneNo"`
	TransactionTrackingRef string `json:"transactionTrackingRef"`
	PlaceOfBirth           string `json:"placeOfBirth"`
	Address                string `json:"address"`
	NationalIdentityNo     string `json:"nationalIdentityNo"`
	NinUserId              string `json:"ninUserId"`
	NextOfKinPhoneNo       string `json:"nextOfKinPhoneNo"`
	NextOfKinName          string `json:"nextOfKinName"`
	Email                  string `json:"email"`
}

// OpenWalletData is the successful response data payload.
type OpenWalletData struct {
	ResponseCode       string `json:"responseCode"`
	OrderRef           string `json:"orderRef"`
	FullName           string `json:"fullName"`
	CreationMessage    string `json:"creationMessage"`
	AccountNumber      string `json:"accountNumber"`
	LedgerBalance      string `json:"ledgerBalance"`
	AvailableBalance   string `json:"availableBalance"`
	CustomerID         string `json:"customerID"`
	Mfbcode            string `json:"mfbcode"`
	FinancialDate      string `json:"financialDate"`
	WithdrawableAmount string `json:"withdrawableAmount"`
}

// statusString unmarshals from either JSON number or string (9PSB may return status as 200 or "200").
type statusString string

func (s *statusString) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		*s = statusString(str)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return err
	}
	*s = statusString(n.String())
	return nil
}

// OpenWalletResponse is the 9PSB open_wallet response.
type OpenWalletResponse struct {
	Status       statusString   `json:"status"`
	ResponseCode string         `json:"responseCode"`
	Message      string         `json:"message"`
	Data         *OpenWalletData `json:"data"`
}

// OpenWalletResult holds the parsed data and full response for storage.
type OpenWalletResult struct {
	Data         *OpenWalletData
	RawResponse  interface{} // full JSON for enc_psb_raw_response
}

// OpenWallet calls 9PSB open_wallet with Bearer token. Returns (result, nil) only when response contains data.accountNumber; otherwise error.
func (p *TokenProvider) OpenWallet(ctx context.Context, body OpenWalletRequest) (*OpenWalletResult, error) {
	token, err := p.GetToken(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+openWalletPath, bytes.NewReader(payload))
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
	var full map[string]interface{}
	if err := json.Unmarshal(raw, &full); err != nil {
		return nil, fmt.Errorf("9PSB open_wallet: invalid JSON: %w", err)
	}
	var out OpenWalletResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("9PSB open_wallet: invalid JSON: %w", err)
	}
	if out.Data == nil || out.Data.AccountNumber == "" {
		log.Printf("9PSB open_wallet: no accountNumber in response | status=%s responseCode=%s message=%q data_nil=%v http_status=%d body=%s",
			out.Status, out.ResponseCode, out.Message, out.Data == nil, resp.StatusCode, string(raw))
		return nil, fmt.Errorf("9PSB open_wallet failed: %s (no accountNumber in response)", out.Message)
	}
	return &OpenWalletResult{Data: out.Data, RawResponse: full}, nil
}
