package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/clients"
	"github.com/abubakvr/payup-backend/services/payment/internal/kafka"
	"github.com/abubakvr/payup-backend/services/payment/internal/psb"
	"github.com/abubakvr/payup-backend/services/payment/internal/repository"
	"github.com/abubakvr/payup-backend/services/payment/internal/validator"
	"github.com/google/uuid"
)

const serviceName = "payment"

var (
	ErrActiveWalletExists = errors.New("active wallet already exists for user")
	ErrKYCNotFound        = errors.New("KYC not found or not complete for user")
)

// PaymentService is the template for payment business logic. Wire in audit logging and SMS (via Kafka) here.
type PaymentService struct {
	repo            *repository.PaymentRepository
	walletRepo      *repository.WalletRepository
	transactionRepo *repository.TransactionRepository
	audit           *kafka.Producer
	notifier        *kafka.Producer
	kycClient       *clients.KYCClient
	userClient      *clients.UserClient
	psbProvider     *psb.TokenProvider
}

// NewPaymentService returns a new payment service.
func NewPaymentService(repo *repository.PaymentRepository, walletRepo *repository.WalletRepository, transactionRepo *repository.TransactionRepository, audit *kafka.Producer, notifier *kafka.Producer, kycClient *clients.KYCClient, userClient *clients.UserClient, psbProvider *psb.TokenProvider) *PaymentService {
	return &PaymentService{
		repo:            repo,
		walletRepo:      walletRepo,
		transactionRepo: transactionRepo,
		audit:           audit,
		notifier:        notifier,
		kycClient:       kycClient,
		userClient:      userClient,
		psbProvider:     psbProvider,
	}
}

// Health returns nil if the service and DB are healthy.
func (s *PaymentService) Health(ctx context.Context) error {
	return s.repo.Health(ctx)
}

// ResolveBeneficiaryResult is the result of resolving a beneficiary (name from 9PSB or other bank).
type ResolveBeneficiaryResult struct {
	Name            string  `json:"name"`
	AccountNumber   string  `json:"account_number"`
	BankCode        string  `json:"bank_code"`
	AvailableBalance float64 `json:"available_balance,omitempty"` // only for 9PSB wallet_enquiry
}

// ResolveBeneficiary returns the account name for the given bank and account number.
// If bank is 9PSB (120001) uses wallet_enquiry; otherwise uses other_banks_enquiry. For frontend to confirm beneficiary exists.
func (s *PaymentService) ResolveBeneficiary(ctx context.Context, bankCode, accountNumber string) (*ResolveBeneficiaryResult, error) {
	const psbBankCode = "120001"
	if s.psbProvider == nil {
		return nil, fmt.Errorf("beneficiary enquiry not configured")
	}
	if bankCode == psbBankCode {
		res, err := s.psbProvider.WalletEnquiry(ctx, accountNumber)
		if err != nil {
			return nil, err
		}
		return &ResolveBeneficiaryResult{
			Name:             res.Name,
			AccountNumber:    res.Nuban,
			BankCode:         bankCode,
			AvailableBalance: res.AvailableBalance,
		}, nil
	}
	name, err := s.psbProvider.OtherBanksEnquiry(ctx, bankCode, accountNumber)
	if err != nil {
		return nil, err
	}
	return &ResolveBeneficiaryResult{
		Name:          name,
		AccountNumber: accountNumber,
		BankCode:      bankCode,
	}, nil
}

// WalletDetails holds wallet fields returned by GET /wallet (from DB).
type WalletDetails struct {
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	Status        string `json:"status"`
}

// GetWalletDetails returns the authenticated user's wallet from the database (account number, account name, status).
func (s *PaymentService) GetWalletDetails(ctx context.Context, userID string) (*WalletDetails, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id")
	}
	wallet, err := s.walletRepo.GetActiveByUserID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, fmt.Errorf("no active wallet")
	}
	return &WalletDetails{
		AccountNumber: wallet.AccountNumber,
		AccountName:   wallet.FullName,
		Status:        "ACTIVE",
	}, nil
}

// GetWalletBalance returns the authenticated user's live balance from 9PSB (wallet_enquiry). Uses user_id to resolve account number from wallet.
func (s *PaymentService) GetWalletBalance(ctx context.Context, userID string) (*psb.WalletEnquiryResult, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id")
	}
	wallet, err := s.walletRepo.GetActiveByUserID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, fmt.Errorf("no active wallet")
	}
	if s.psbProvider == nil {
		return nil, fmt.Errorf("wallet enquiry not configured")
	}
	return s.psbProvider.WalletEnquiry(ctx, wallet.AccountNumber)
}

// GetWalletTransactionHistory returns the authenticated user's wallet transaction history (newest first). limit/offset for pagination.
func (s *PaymentService) GetWalletTransactionHistory(ctx context.Context, userID string, limit, offset int) ([]repository.TransactionHistoryRow, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id")
	}
	wallet, err := s.walletRepo.GetActiveByUserID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, fmt.Errorf("no active wallet")
	}
	return s.transactionRepo.ListByWalletID(ctx, wallet.WalletID, limit, offset)
}

// GetTransactionDetail returns a single transaction by transaction_ref for the authenticated user's wallet, or nil if not found.
func (s *PaymentService) GetTransactionDetail(ctx context.Context, userID string, transactionRef string) (*repository.TransactionHistoryRow, error) {
	if transactionRef == "" {
		return nil, nil
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id")
	}
	wallet, err := s.walletRepo.GetActiveByUserID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, fmt.Errorf("no active wallet")
	}
	return s.transactionRepo.GetByRefAndWalletID(ctx, transactionRef, wallet.WalletID)
}

// ListWalletsForAdmin returns all wallets with decrypted details for admin. limit/offset for pagination (limit default 50 max 100).
func (s *PaymentService) ListWalletsForAdmin(ctx context.Context, limit, offset int) ([]repository.WalletAdminRow, error) {
	if s.walletRepo == nil {
		return nil, fmt.Errorf("wallet repository not configured")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.walletRepo.ListForAdmin(ctx, limit, offset)
}

// SendAuditLog sends an audit log to the audit service via Kafka. Use from your payment handlers.
func (s *PaymentService) SendAuditLog(params kafka.AuditLogParams) error {
	if s.audit == nil {
		return nil
	}
	params.Service = serviceName
	return s.audit.SendAuditLog(params)
}

// SendNotification sends an event to the notification service (SMS, email, etc.) via Kafka.
func (s *PaymentService) SendNotification(ev kafka.NotificationEvent) error {
	if s.notifier == nil {
		return nil
	}
	return s.notifier.SendNotification(ev)
}

// CreateWallet creates a 9PSB wallet for the user. Fetches KYC via gRPC, calls 9PSB open_wallet, stores in DB only on success.
func (s *PaymentService) CreateWallet(ctx context.Context, userID string) (accountNumber string, err error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("invalid user_id: %w", err)
	}
	if s.walletRepo == nil {
		return "", fmt.Errorf("wallet repository not configured")
	}
	has, err := s.walletRepo.HasActiveWallet(ctx, uid)
	if err != nil {
		return "", err
	}
	if has {
		return "", ErrActiveWalletExists
	}
	if s.kycClient == nil {
		return "", fmt.Errorf("KYC client not configured")
	}
	kycResp, err := s.kycClient.GetKYCForWallet(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get KYC for wallet: %w", err)
	}
	if kycResp == nil || !kycResp.Found {
		return "", ErrKYCNotFound
	}
	email := kycResp.Email
	if email == "" && s.userClient != nil {
		if u, _ := s.userClient.GetUserForKYC(ctx, userID); u != nil && u.Found {
			email = u.Email
		}
	}
	trackingRef := generateTrackingRef("WLT")
	req, err := validator.ValidateAndSanitizeOpenWalletInput(kycResp, email, trackingRef)
	if err != nil {
		return "", fmt.Errorf("wallet open validation: %w", err)
	}
	if s.psbProvider == nil {
		return "", fmt.Errorf("9PSB token provider not configured")
	}
	result, err := s.psbProvider.OpenWallet(ctx, req)
	if err != nil {
		return "", err
	}
	// Store only after successful 9PSB response with accountNumber
	row := &repository.WalletRow{
		UserID:           uid,
		AccountNumber:    result.Data.AccountNumber,
		CustomerID:       result.Data.CustomerID,
		OrderRef:         result.Data.OrderRef,
		FullName:         result.Data.FullName,
		Phone:            req.PhoneNo,
		Email:            req.Email,
		MfbCode:          result.Data.Mfbcode,
		Tier:             "1",
		Status:           "ACTIVE",
		LedgerBalance:    0,
		AvailableBalance: 0,
		Provider:         "9PSB",
		PsbRawResponse:   result.RawResponse,
	}
	if err := s.walletRepo.Create(ctx, row); err != nil {
		return "", fmt.Errorf("store wallet: %w", err)
	}

	// Audit: wallet created
	_ = s.SendAuditLog(kafka.AuditLogParams{
		Action:   "wallet_created",
		Entity:   "wallet",
		EntityID: result.Data.AccountNumber,
		UserID:   &userID,
		Metadata: map[string]interface{}{
			"account_number": result.Data.AccountNumber,
			"order_ref":      result.Data.OrderRef,
			"provider":      "9PSB",
		},
	})

	// Congratulatory email: wallet opened, account number, top up and login
	if req.Email != "" {
		_ = s.SendNotification(kafka.NotificationEvent{
			Type:    "wallet_opened",
			Channel: "email",
			Metadata: map[string]interface{}{
				"to":      req.Email,
				"subject": "Your PayUp wallet is ready",
				"html":    buildWalletOpenedEmailHTML(result.Data.AccountNumber),
			},
		})
		_ = s.SendAuditLog(kafka.AuditLogParams{
			Service:  "payment",
			Action:   "wallet_opened_email_sent",
			Entity:   "wallet",
			EntityID: result.Data.AccountNumber,
			UserID:   &userID,
			Metadata: map[string]interface{}{"to": req.Email},
		})
	}

	// Notify KYC to set kyc_level=1 (wallet created)
	if s.audit != nil {
		_ = s.audit.PublishWalletCreated(ctx, userID)
	}

	return result.Data.AccountNumber, nil
}

// buildWalletOpenedEmailHTML returns HTML body for the wallet-opened congratulatory email.
func buildWalletOpenedEmailHTML(accountNumber string) string {
	return `<p>Congratulations! Your PayUp wallet has been successfully opened.</p>` +
		`<p><strong>Your account number:</strong> ` + accountNumber + `</p>` +
		`<p>You can now:</p>` +
		`<ul>` +
		`<li>Top up your wallet to start making payments</li>` +
		`<li>Log in to your account to view your balance and transaction history</li>` +
		`</ul>` +
		`<p>Thank you for choosing PayUp.</p>`
}

func generateTrackingRef(prefix string) string {
	return prefix + time.Now().Format("20060102150405") + uuid.New().String()[:8]
}
