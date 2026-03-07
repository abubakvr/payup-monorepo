package psb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
)

const (
	waasWalletTransactionsPath     = "/api/v1/wallet_transactions"
	waasWalletStatusPath           = "/api/v1/wallet_status"
	waasChangeWalletStatusPath     = "/api/v1/change_wallet_status"
	waasWalletUpgradeFileUploadPath = "/api/v1/wallet_upgrade_file_upload"
	waasUpgradeStatusPath          = "/api/v1/upgrade_status"
)

// WaasWalletTransactionsRequest is the request body for 9PSB WaaS wallet_transactions.
type WaasWalletTransactionsRequest struct {
	AccountNumber  string `json:"accountNumber"`
	FromDate       string `json:"fromDate"`       // YYYY-MM-DD
	ToDate         string `json:"toDate"`         // YYYY-MM-DD (max 31 days from fromDate)
	NumberOfItems  string `json:"numberOfItems"`  // e.g. "20"
}

// WaasWalletTransactionItem is a single transaction in the 9PSB WaaS transaction history.
type WaasWalletTransactionItem struct {
	TransactionDate      string   `json:"transactionDate"`
	AccountNumber        *string  `json:"accountNumber"`
	Amount               float64  `json:"amount"`
	Narration            string   `json:"narration"`
	IsReversed           bool     `json:"isReversed"`
	TransactionDateString string   `json:"transactionDateString"`
	Balance              float64  `json:"balance"`
	ReferenceID          string   `json:"referenceID"`
	PostingType          string   `json:"postingType"`
	Debit                string   `json:"debit"`
	Credit               string   `json:"credit"`
	ReversalReferenceNo  *string  `json:"reversalReferenceNo"`
	UniqueIdentifier     string   `json:"uniqueIdentifier"`
	CurrentDate          string   `json:"currentDate"`
	IsCardTransation     bool     `json:"isCardTransation"`
}

// WaasWalletTransactionsResponse is the 9PSB WaaS wallet_transactions response.
type WaasWalletTransactionsResponse struct {
	Status  string `json:"status"`  // "SUCCESS" or "FAILED"
	Message string `json:"message"`
	Data    struct {
		ResponseCode string                      `json:"responseCode"`
		Successful   bool                        `json:"successful"`
		Message     []WaasWalletTransactionItem `json:"message"` // list of transactions when successful
	} `json:"data"`
}

// WaasWalletTransactions calls 9PSB WaaS wallet_transactions. Date range must not exceed 31 days.
func (p *TokenProvider) WaasWalletTransactions(ctx context.Context, accountNumber, fromDate, toDate, numberOfItems string) (*WaasWalletTransactionsResponse, error) {
	body := WaasWalletTransactionsRequest{
		AccountNumber: accountNumber,
		FromDate:      fromDate,
		ToDate:        toDate,
		NumberOfItems: numberOfItems,
	}
	raw, statusCode, err := p.waasPost(ctx, waasWalletTransactionsPath, body)
	if err != nil {
		return nil, err
	}
	var out WaasWalletTransactionsResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("9PSB WaaS wallet_transactions invalid JSON (HTTP %d): %w", statusCode, err)
	}
	if out.Status != "SUCCESS" {
		return &out, fmt.Errorf("9PSB WaaS wallet_transactions: %s", out.Message)
	}
	n := 0
	if out.Data.Message != nil {
		n = len(out.Data.Message)
	}
	log.Printf("9PSB WaaS wallet_transactions: accountNumber=***%s fromDate=%s toDate=%s count=%d", maskAccount(accountNumber), fromDate, toDate, n)
	return &out, nil
}

func maskAccount(accountNumber string) string {
	if len(accountNumber) <= 4 {
		return accountNumber
	}
	return accountNumber[len(accountNumber)-4:]
}

// WaasWalletStatusRequest is the request body for 9PSB WaaS wallet_status.
type WaasWalletStatusRequest struct {
	AccountNo string `json:"accountNo"`
}

// WaasWalletStatusResponse is the 9PSB WaaS wallet_status response.
type WaasWalletStatusResponse struct {
	Status  string `json:"status"`  // "SUCCESS" or "FAILED"
	Message string `json:"message"`
	Data    struct {
		WalletStatus string `json:"walletStatus"` // e.g. "ACTIVE"
		ResponseCode string `json:"responseCode"`
	} `json:"data"`
}

// WaasWalletStatus calls 9PSB WaaS wallet_status.
func (p *TokenProvider) WaasWalletStatus(ctx context.Context, accountNo string) (*WaasWalletStatusResponse, error) {
	body := WaasWalletStatusRequest{AccountNo: accountNo}
	raw, statusCode, err := p.waasPost(ctx, waasWalletStatusPath, body)
	if err != nil {
		return nil, err
	}
	var out WaasWalletStatusResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("9PSB WaaS wallet_status invalid JSON (HTTP %d): %w", statusCode, err)
	}
	if out.Status != "SUCCESS" {
		return &out, fmt.Errorf("9PSB WaaS wallet_status: %s", out.Message)
	}
	return &out, nil
}

// WaasUpgradeStatusRequest is the request body for 9PSB WaaS upgrade_status.
type WaasUpgradeStatusRequest struct {
	AccountNumber string `json:"accountNumber"`
}

// WaasUpgradeStatusResponse is the 9PSB WaaS upgrade_status response. Use for user/admin upgrade status (source of truth from 9PSB).
type WaasUpgradeStatusResponse struct {
	Status  string `json:"status"`  // "SUCCESS" or "FAILED"
	Message string `json:"message"`
	Data    struct {
		Message string `json:"message"` // e.g. "Pending", "No record found"
		Status  string `json:"status"`  // e.g. "Successful", "Failed"
	} `json:"data"`
}

// WaasUpgradeStatus calls 9PSB WaaS upgrade_status. Returns the parsed response; caller should use response.Status and response.Data for display (no error for "No record found").
func (p *TokenProvider) WaasUpgradeStatus(ctx context.Context, accountNumber string) (*WaasUpgradeStatusResponse, error) {
	body := WaasUpgradeStatusRequest{AccountNumber: accountNumber}
	raw, statusCode, err := p.waasPost(ctx, waasUpgradeStatusPath, body)
	if err != nil {
		return nil, err
	}
	var out WaasUpgradeStatusResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("9PSB WaaS upgrade_status invalid JSON (HTTP %d): %w", statusCode, err)
	}
	return &out, nil
}

// WaasChangeWalletStatusRequest is the request body for 9PSB WaaS change_wallet_status.
type WaasChangeWalletStatusRequest struct {
	AccountNumber string `json:"accountNumber"`
	AccountStatus string `json:"accountStatus"` // "ACTIVE" or "SUSPENDED"
}

// WaasChangeWalletStatusResponse is the 9PSB WaaS change_wallet_status response.
type WaasChangeWalletStatusResponse struct {
	Status       string `json:"status"`       // "SUCCESS" or "FAILED"
	ResponseCode string `json:"responseCode"`
	Message      string `json:"message"`
	Data         struct {
		NewWalletStatus string `json:"newWalletStatus"`
		ResponseCode    string `json:"responseCode"`
	} `json:"data"`
}

// WaasChangeWalletStatus calls 9PSB WaaS change_wallet_status. accountStatus must be "ACTIVE" or "SUSPENDED".
func (p *TokenProvider) WaasChangeWalletStatus(ctx context.Context, accountNumber, accountStatus string) (*WaasChangeWalletStatusResponse, error) {
	if accountStatus != "ACTIVE" && accountStatus != "SUSPENDED" {
		return nil, fmt.Errorf("accountStatus must be ACTIVE or SUSPENDED")
	}
	body := WaasChangeWalletStatusRequest{AccountNumber: accountNumber, AccountStatus: accountStatus}
	raw, statusCode, err := p.waasPost(ctx, waasChangeWalletStatusPath, body)
	if err != nil {
		return nil, err
	}
	var out WaasChangeWalletStatusResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("9PSB WaaS change_wallet_status invalid JSON (HTTP %d): %w", statusCode, err)
	}
	if out.Status != "SUCCESS" {
		return &out, fmt.Errorf("9PSB WaaS change_wallet_status: %s", out.Message)
	}
	return &out, nil
}

// WaasWalletUpgradeFormFields holds text fields for 9PSB wallet_upgrade_file_upload (multipart). ChannelType: MOBILE | WEB | USSD | AGENT.
type WaasWalletUpgradeFormFields struct {
	AccountName     string
	AccountNumber   string
	BVN             string
	ChannelType     string // MOBILE | WEB | USSD | AGENT (admin-initiated use AGENT)
	City            string
	Email           string
	HouseNumber     string
	IDIssueDate     string // YYYY-MM-DD
	IDNumber        string
	IDType          string // 1, 2, 3
	LocalGovernment string
	PEP             string // YES or NO
	PhoneNumber     string
	State           string
	StreetName      string
	Tier            string
	IDExpiryDate    string // YYYY-MM-DD
	NearestLandmark string
	PlaceOfBirth    string
	NIN             string
}

// WaasWalletUpgradeFileUploadResponse is the 9PSB wallet_upgrade_file_upload JSON response.
type WaasWalletUpgradeFileUploadResponse struct {
	Status  string `json:"status"`  // "SUCCESS" or "FAILED"
	Message string `json:"message"`
	Data    struct {
		Message      string `json:"message"`
		Status       string `json:"status"`
		ResponseCode string `json:"responseCode"`
	} `json:"data"`
}

// WaasWalletUpgradeFileUpload sends a multipart/form-data request to 9PSB wallet_upgrade_file_upload. Image parts: idCardFront, idCardBack, customerImage, userPhoto, utilityBill, proofOfAddressVerification (JPEG/PNG bytes).
func (p *TokenProvider) WaasWalletUpgradeFileUpload(ctx context.Context, form *WaasWalletUpgradeFormFields, idFrontImage, idBackImage, customerImage, utilityBillImage, proofOfAddressVerificationImage []byte) (*WaasWalletUpgradeFileUploadResponse, error) {
	if p.waasBaseURL == "" {
		return nil, fmt.Errorf("9PSB WaaS base URL not configured")
	}
	token, err := p.GetToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("9PSB auth: %w", err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// 9PSB returns "must not be blank" if any required field is empty; never send blank.
	nonBlank := func(s, fallback string) string {
		if s == "" {
			return fallback
		}
		return s
	}
	writeField := func(name, value string) {
		if value == "" {
			value = "N/A"
		}
		_ = w.WriteField(name, value)
	}
	writeField("accountName", nonBlank(form.AccountName, "N/A"))
	writeField("accountNumber", nonBlank(form.AccountNumber, "N/A"))
	writeField("bvn", nonBlank(form.BVN, "N/A"))
	writeField("channelType", nonBlank(form.ChannelType, "AGENT"))
	writeField("city", nonBlank(form.City, "N/A"))
	writeField("email", nonBlank(form.Email, "N/A"))
	writeField("houseNumber", nonBlank(form.HouseNumber, "N/A"))
	writeField("idIssueDate", nonBlank(form.IDIssueDate, "2015-08-11"))
	writeField("idNumber", nonBlank(form.IDNumber, "N/A"))
	writeField("idType", nonBlank(form.IDType, "1"))
	writeField("localGovernment", nonBlank(form.LocalGovernment, "N/A"))
	writeField("pep", nonBlank(form.PEP, "NO"))
	writeField("phoneNumber", nonBlank(form.PhoneNumber, "N/A"))
	writeField("state", nonBlank(form.State, "N/A"))
	writeField("streetName", nonBlank(form.StreetName, "N/A"))
	writeField("tier", nonBlank(form.Tier, "3"))
	writeField("idExpiryDate", nonBlank(form.IDExpiryDate, "2028-09-12"))
	writeField("nearestLandmark", nonBlank(form.NearestLandmark, "N/A"))
	writeField("placeOfBirth", nonBlank(form.PlaceOfBirth, "N/A"))
	writeField("nin", nonBlank(form.NIN, "N/A"))

	contentTypeForImage := func(data []byte) string {
		if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
			return "image/jpeg"
		}
		if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
			return "image/png"
		}
		return "image/jpeg"
	}
	addFilePart := func(fieldName, filename string, data []byte) {
		if len(data) == 0 {
			return
		}
		ct := contentTypeForImage(data)
		h := textproto.MIMEHeader{
			"Content-Disposition": {fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filename)},
			"Content-Type":       {ct},
		}
		pw, _ := w.CreatePart(h)
		_, _ = pw.Write(data)
	}
	// 9PSB multipart field names (idCardFront/idCardBack required)
	addFilePart("idCardFront", "id_front.jpg", idFrontImage)
	addFilePart("idCardBack", "id_back.jpg", idBackImage)
	addFilePart("customerImage", "customer.jpg", customerImage)
	addFilePart("userPhoto", "user_photo.jpg", customerImage)
	addFilePart("utilityBill", "utility_bill.jpg", utilityBillImage)
	addFilePart("proofOfAddressVerification", "proof_of_address.jpg", proofOfAddressVerificationImage)

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("multipart close: %w", err)
	}

	contentType := w.FormDataContentType()
	url := p.waasBaseURL + waasWalletUpgradeFileUploadPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)

	client := p.httpClient
	if p.waasClient != nil {
		client = p.waasClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("9PSB WaaS wallet_upgrade_file_upload request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("9PSB WaaS wallet_upgrade_file_upload response (HTTP %d): %w", resp.StatusCode, err)
	}
	var out WaasWalletUpgradeFileUploadResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("9PSB WaaS wallet_upgrade_file_upload invalid JSON (HTTP %d): %w", resp.StatusCode, err)
	}
	if out.Status != "SUCCESS" {
		log.Printf("9PSB WaaS wallet_upgrade_file_upload failed: accountNumber=***%s status=%s message=%s", maskAccount(form.AccountNumber), out.Status, out.Message)
		return &out, fmt.Errorf("9PSB wallet upgrade: %s", out.Message)
	}
	log.Printf("9PSB WaaS wallet_upgrade_file_upload success: accountNumber=***%s", maskAccount(form.AccountNumber))
	return &out, nil
}
