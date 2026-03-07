package psb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/repository"
)

const (
	// 9PSB Bank9ja API v2 auth path (baseURL + this path)
	authenticatePath = "/api/v1/authenticate"
	reuseBuffer      = 2 * time.Minute
)

// AuthenticateResponse is the 9PSB authenticate response (accessToken, refreshToken, expiresIn, etc.).
type AuthenticateResponse struct {
	Message          string `json:"message"`
	AccessToken      string `json:"accessToken"`
	ExpiresIn        string `json:"expiresIn"` // e.g. "300" (seconds)
	RefreshToken     string `json:"refreshToken"`
	RefreshExpiresIn string `json:"refreshExpiresIn"` // e.g. "1800"
	JWT              string `json:"jwt"`
	TokenType        string `json:"tokenType"`
}

// TokenProvider returns a valid Bearer token for 9PSB API calls. Tokens are stored in DB and reused until near-expiry.
type TokenProvider struct {
	baseURL      string
	baseURL2     string // optional; for wallet_other_banks if different from baseURL
	waasBaseURL  string // optional; for WaaS debit/credit e.g. http://102.216.128.75:9090/waas
	username     string
	password     string
	clientID     string
	clientSecret string
	authRepo    *repository.AuthTokenRepository
	httpClient  *http.Client
	waasClient  *http.Client // uses custom transport to tolerate duplicate Transfer-Encoding in 9PSB response
	mu          sync.Mutex
}

// NewTokenProvider creates a token provider that uses the auth_tokens table (encrypted). baseURL2 is optional (for wallet_other_banks). waasBaseURL is optional (for WaaS debit/credit).
func NewTokenProvider(baseURL, baseURL2, waasBaseURL, username, password, clientID, clientSecret string, authRepo *repository.AuthTokenRepository) *TokenProvider {
	u2 := baseURL2
	if u2 == "" {
		u2 = baseURL
	}
	httpClient := &http.Client{Timeout: 30 * time.Second}
	var waasClient *http.Client
	if waasBaseURL != "" {
		waasHost := waasHostFromURL(waasBaseURL)
		if waasHost != "" {
			waasClient = &http.Client{
				Timeout:   45 * time.Second,
				Transport: newWaasRoundTripper(waasHost, 45*time.Second),
			}
		}
	}
	return &TokenProvider{
		baseURL:      baseURL,
		baseURL2:     u2,
		waasBaseURL:  waasBaseURL,
		username:     username,
		password:     password,
		clientID:     clientID,
		clientSecret: clientSecret,
		authRepo:     authRepo,
		httpClient:   httpClient,
		waasClient:   waasClient,
	}
}

// GetToken returns a valid access token. Reuses cached token from DB if it has more than 2 minutes until expiry; otherwise re-authenticates and upserts.
func (p *TokenProvider) GetToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	row, err := p.authRepo.GetByClientID(ctx, p.clientID)
	if err != nil {
		return "", err
	}
	if row != nil && time.Until(row.ExpiresAt) > reuseBuffer {
		return row.AccessToken, nil
	}

	// Re-authenticate
	body := map[string]string{
		"username":     p.username,
		"password":     p.password,
		"clientId":     p.clientID,
		"clientSecret": p.clientSecret,
	}
	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+authenticatePath, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("9PSB authenticate: status %d body %s", resp.StatusCode, string(raw))
	}

	var data AuthenticateResponse
	if err := json.Unmarshal(raw, &data); err != nil {
		return "", err
	}
	if data.Message != "successful" {
		return "", fmt.Errorf("9PSB authenticate: message=%s", data.Message)
	}

	expiresIn := parseInt(data.ExpiresIn, 300)
	refreshExpiresIn := parseInt(data.RefreshExpiresIn, 1800)
	tokenType := data.TokenType
	if tokenType == "" {
		tokenType = "Bearer"
	}

	if err := p.authRepo.Upsert(ctx, p.clientID, data.AccessToken, data.RefreshToken, tokenType, expiresIn, refreshExpiresIn); err != nil {
		return "", err
	}
	return data.AccessToken, nil
}

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return n
}

func waasHostFromURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}
