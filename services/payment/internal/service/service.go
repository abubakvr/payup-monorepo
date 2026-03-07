package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/clients"
	"github.com/abubakvr/payup-backend/services/payment/internal/crypto"
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
	repo                *repository.PaymentRepository
	walletRepo          *repository.WalletRepository
	walletUpgradeRepo   *repository.WalletUpgradeRepository
	webhookEventsRepo   *repository.WebhookEventsRepository
	transactionRepo     *repository.TransactionRepository
	audit               *kafka.Producer
	notifier            *kafka.Producer
	kycClient           *clients.KYCClient
	userClient          *clients.UserClient
	psbProvider         *psb.TokenProvider
}

// NewPaymentService returns a new payment service.
func NewPaymentService(repo *repository.PaymentRepository, walletRepo *repository.WalletRepository, walletUpgradeRepo *repository.WalletUpgradeRepository, webhookEventsRepo *repository.WebhookEventsRepository, transactionRepo *repository.TransactionRepository, audit *kafka.Producer, notifier *kafka.Producer, kycClient *clients.KYCClient, userClient *clients.UserClient, psbProvider *psb.TokenProvider) *PaymentService {
	return &PaymentService{
		repo:              repo,
		walletRepo:        walletRepo,
		walletUpgradeRepo: walletUpgradeRepo,
		webhookEventsRepo: webhookEventsRepo,
		transactionRepo:   transactionRepo,
		audit:             audit,
		notifier:          notifier,
		kycClient:         kycClient,
		userClient:        userClient,
		psbProvider:       psbProvider,
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

// GetWaasTransactionHistory returns 9PSB WaaS transaction history for the user's wallet. Date range max 31 days.
func (s *PaymentService) GetWaasTransactionHistory(ctx context.Context, userID, fromDate, toDate string, limit int) (*psb.WaasWalletTransactionsResponse, error) {
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
		return nil, fmt.Errorf("9PSB WaaS not configured")
	}
	numItems := fmt.Sprintf("%d", limit)
	if numItems == "0" {
		numItems = "20"
	}
	return s.psbProvider.WaasWalletTransactions(ctx, wallet.AccountNumber, fromDate, toDate, numItems)
}

// GetWaasWalletStatus returns 9PSB WaaS wallet status for the user's wallet.
func (s *PaymentService) GetWaasWalletStatus(ctx context.Context, userID string) (*psb.WaasWalletStatusResponse, error) {
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
		return nil, fmt.Errorf("9PSB WaaS not configured")
	}
	return s.psbProvider.WaasWalletStatus(ctx, wallet.AccountNumber)
}

// ChangeWalletStatusResult is the result of changing a user's wallet status via 9PSB.
type ChangeWalletStatusResult struct {
	NewWalletStatus string
}

// ChangeWalletStatus changes the user's wallet status via 9PSB WaaS (ACTIVE or SUSPENDED), updates DB, sends audit and email.
func (s *PaymentService) ChangeWalletStatus(ctx context.Context, userID, newStatus, initiatedBy string) (*ChangeWalletStatusResult, error) {
	if newStatus != "ACTIVE" && newStatus != "SUSPENDED" {
		return nil, fmt.Errorf("status must be ACTIVE or SUSPENDED")
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id")
	}
	wallet, err := s.walletRepo.GetByUserIDForStatusChange(ctx, uid)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, fmt.Errorf("no wallet found for user")
	}
	if s.psbProvider == nil {
		return nil, fmt.Errorf("9PSB WaaS not configured")
	}
	resp, err := s.psbProvider.WaasChangeWalletStatus(ctx, wallet.AccountNumber, newStatus)
	if err != nil {
		return nil, err
	}
	newStatusDB := resp.Data.NewWalletStatus
	if newStatusDB == "" {
		newStatusDB = newStatus
	}
	if err := s.walletRepo.UpdateStatus(ctx, wallet.WalletID, newStatusDB); err != nil {
		return nil, fmt.Errorf("updated 9PSB but failed to update DB: %w", err)
	}
	if s.audit != nil {
		_ = s.SendAuditLog(kafka.AuditLogParams{
			Service:  serviceName,
			Action:   "admin_wallet_status_changed",
			Entity:   "wallet",
			EntityID: userID,
			UserID:   strPtr(initiatedBy),
			Metadata: map[string]interface{}{
				"user_id":         userID,
				"previous_status": wallet.CurrentStatus,
				"new_status":      newStatusDB,
			},
		})
	}
	var toEmail string
	if s.userClient != nil {
		if u, _ := s.userClient.GetUserForKYC(ctx, userID); u != nil && u.Found {
			toEmail = u.Email
		}
	}
	if toEmail != "" && s.notifier != nil {
		subject := "Your PayUp wallet has been activated"
		html := buildWalletStatusActivatedEmailHTML()
		if newStatusDB == "SUSPENDED" {
			subject = "Your PayUp wallet has been suspended"
			html = buildWalletStatusSuspendedEmailHTML()
		}
		_ = s.SendNotification(kafka.NotificationEvent{
			Type:    "wallet_status_changed",
			Channel: "email",
			Metadata: map[string]interface{}{
				"to":      toEmail,
				"subject": subject,
				"html":    html,
				"status":  newStatusDB,
			},
		})
	}
	return &ChangeWalletStatusResult{NewWalletStatus: newStatusDB}, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func buildWalletStatusActivatedEmailHTML() string {
	return `<p>Your PayUp wallet has been activated and you can use it for transactions again.</p><p>Thank you for using PayUp.</p>`
}

func buildWalletStatusSuspendedEmailHTML() string {
	return `<p>Your PayUp wallet has been suspended. You will not be able to perform transactions until it is reactivated.</p><p>If you have questions, please contact support.</p><p>Thank you for using PayUp.</p>`
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

// SubmitWalletUpgradeResult is the result of submitting a wallet upgrade to 9PSB.
type SubmitWalletUpgradeResult struct {
	Message string // e.g. "Request submitted successfully"
}

// SubmitWalletUpgrade fetches KYC + images from KYC service, sends multipart to 9PSB wallet_upgrade_file_upload, then audit + email via Kafka.
func (s *PaymentService) SubmitWalletUpgrade(ctx context.Context, userID, initiatedBy string) (*SubmitWalletUpgradeResult, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id")
	}
	wallet, err := s.walletRepo.GetActiveByUserID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if wallet == nil {
		return nil, fmt.Errorf("no active wallet for user")
	}
	if s.kycClient == nil {
		return nil, fmt.Errorf("KYC client not configured")
	}
	kycResp, err := s.kycClient.GetKYCForWalletUpgrade(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("KYC for wallet upgrade: %w", err)
	}
	if kycResp == nil || !kycResp.Found {
		return nil, fmt.Errorf("KYC data not found for user")
	}
	if s.psbProvider == nil {
		return nil, fmt.Errorf("9PSB WaaS not configured")
	}
	accountName := wallet.FullName
	if accountName == "" {
		accountName = kycResp.AccountName
	}
	// 9PSB expects dates in yyyy-MM-dd only; default when empty or wrong format
	idIssueDate := normalizeDateFor9PSB(kycResp.IdIssueDate, "2015-08-11")
	idExpiryDate := normalizeDateFor9PSB(kycResp.IdExpiryDate, "2028-09-12")
	form := &psb.WaasWalletUpgradeFormFields{
		AccountName:     nonBlank(accountName, "N/A"),
		AccountNumber:   wallet.AccountNumber,
		BVN:             nonBlank(kycResp.Bvn, "N/A"),
		ChannelType:     "AGENT",
		City:            nonBlank(kycResp.City, "N/A"),
		Email:           nonBlank(kycResp.Email, "N/A"),
		HouseNumber:     nonBlank(kycResp.HouseNumber, "N/A"),
		IDIssueDate:     idIssueDate,
		IDNumber:        nonBlank(kycResp.IdNumber, "N/A"),
		IDType:          nonBlank(kycResp.IdType, "1"),
		LocalGovernment: nonBlank(kycResp.LocalGovernment, "N/A"),
		PEP:             nonBlank(kycResp.Pep, "NO"),
		PhoneNumber:     nonBlank(kycResp.PhoneNumber, "N/A"),
		State:           nonBlank(kycResp.State, "N/A"),
		StreetName:      nonBlank(kycResp.StreetName, "N/A"),
		Tier:            nonBlank(kycResp.Tier, "3"),
		IDExpiryDate:    idExpiryDate,
		NearestLandmark: nonBlank(kycResp.NearestLandmark, "N/A"),
		PlaceOfBirth:    nonBlank(kycResp.PlaceOfBirth, "N/A"),
		NIN:             nonBlank(kycResp.Nin, "N/A"),
	}
	// 9PSB requires all document images; ensure KYC returned them (user must complete identity + address verification).
	if len(kycResp.IdFrontImage) == 0 {
		return nil, fmt.Errorf("ID card front image is required for wallet upgrade; ensure the user has uploaded identity documents (ID front) in KYC")
	}
	if len(kycResp.IdBackImage) == 0 {
		return nil, fmt.Errorf("ID card back image is required for wallet upgrade; ensure the user has uploaded identity documents (ID back) in KYC")
	}
	if len(kycResp.CustomerImage) == 0 {
		return nil, fmt.Errorf("customer/selfie image is required for wallet upgrade; ensure the user has completed BVN verification or uploaded identity customer image in KYC")
	}
	hasProofOfAddress := len(kycResp.UtilityBillImage) > 0
	if !hasProofOfAddress {
		return nil, fmt.Errorf("utility bill image is required for wallet upgrade; ensure the user has completed address verification with a utility bill")
	}
	if len(kycResp.ProofOfAddressImage) == 0 {
		return nil, fmt.Errorf("proof of address verification image is required for Tier 3 upgrade; ensure the user has uploaded proof of address in KYC address verification")
	}
	tierFrom := wallet.Tier
	if tierFrom != "1" && tierFrom != "2" {
		tierFrom = "1"
	}
	accountNumberHash := crypto.FieldHash(wallet.AccountNumber)
	var upgradeID uuid.UUID
	if s.walletUpgradeRepo != nil {
		upgradeID, _, err = s.walletUpgradeRepo.CreatePending(ctx, wallet.WalletID, accountNumberHash, tierFrom, "3", form, initiatedBy, hasProofOfAddress)
		if err != nil {
			return nil, fmt.Errorf("create upgrade request: %w", err)
		}
	}
	resp, err := s.psbProvider.WaasWalletUpgradeFileUpload(ctx, form, kycResp.IdFrontImage, kycResp.IdBackImage, kycResp.CustomerImage, kycResp.UtilityBillImage, kycResp.ProofOfAddressImage)
	now := time.Now()
	if s.walletUpgradeRepo != nil && upgradeID != uuid.Nil {
		if err != nil {
			_ = s.walletUpgradeRepo.UpdateAfter9PSB(ctx, upgradeID, "FAILED", "", nil, now)
		} else {
			psbCode := ""
			if resp != nil {
				psbCode = resp.Data.ResponseCode
			}
			_ = s.walletUpgradeRepo.UpdateAfter9PSB(ctx, upgradeID, "SUBMITTED", psbCode, resp, now)
		}
	}
	if err != nil {
		return nil, err
	}
	msg := resp.Data.Message
	if msg == "" {
		msg = resp.Message
	}
	if s.audit != nil {
		_ = s.SendAuditLog(kafka.AuditLogParams{
			Service:  serviceName,
			Action:   "admin_wallet_upgrade_submitted",
			Entity:   "wallet",
			EntityID: userID,
			UserID:   strPtr(initiatedBy),
			Metadata: map[string]interface{}{
				"user_id":        userID,
				"account_number": wallet.AccountNumber,
				"message":        msg,
			},
		})
	}
	var toEmail string
	if s.userClient != nil {
		if u, _ := s.userClient.GetUserForKYC(ctx, userID); u != nil && u.Found {
			toEmail = u.Email
		}
	}
	if toEmail != "" && s.notifier != nil {
		_ = s.SendNotification(kafka.NotificationEvent{
			Type:    "wallet_upgrade_submitted",
			Channel: "email",
			Metadata: map[string]interface{}{
				"to":      toEmail,
				"subject": "Wallet upgrade request submitted",
				"html":    buildWalletUpgradeSubmittedEmailHTML(),
			},
		})
	}
	return &SubmitWalletUpgradeResult{Message: msg}, nil
}

// nonBlank returns s if non-empty after trim, else fallback. Use for 9PSB form fields that must not be blank.
func nonBlank(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return strings.TrimSpace(s)
}

// normalizeDateFor9PSB returns a date in yyyy-MM-dd for 9PSB. Uses default when input is empty or not in a supported format.
func normalizeDateFor9PSB(input, defaultVal string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal
	}
	// Already yyyy-MM-dd (e.g. 2015-08-11)
	if len(input) >= 10 && input[4] == '-' && input[7] == '-' {
		return input[:10]
	}
	// DD/MM/YYYY or DD/MM/YY
	if len(input) >= 10 && (input[2] == '/' && input[5] == '/') {
		parts := strings.Split(input, "/")
		if len(parts) == 3 {
			y, d := parts[2], parts[0]
			if len(y) == 2 {
				y = "20" + y
			}
			m := parts[1]
			if len(d) == 1 {
				d = "0" + d
			}
			if len(m) == 1 {
				m = "0" + m
			}
			return y + "-" + m + "-" + d
		}
	}
	return defaultVal
}

func buildWalletUpgradeSubmittedEmailHTML() string {
	return `<p>Your wallet upgrade request has been submitted successfully to 9PSB.</p><p>You will be notified once the upgrade is processed.</p><p>Thank you for using PayUp.</p>`
}

// ListWalletUpgradeRequests returns wallet upgrade requests for admin (paginated). Newest first.
func (s *PaymentService) ListWalletUpgradeRequests(ctx context.Context, limit, offset int) ([]repository.WalletUpgradeRequestRow, error) {
	if s.walletUpgradeRepo == nil {
		return nil, fmt.Errorf("wallet upgrade repository not configured")
	}
	return s.walletUpgradeRepo.ListWalletUpgradeRequests(ctx, limit, offset)
}

// GetWalletUpgradeRequestByID returns one wallet upgrade request by id (with decrypted request/response payloads for admin detail).
func (s *PaymentService) GetWalletUpgradeRequestByID(ctx context.Context, id string) (*repository.WalletUpgradeRequestDetail, error) {
	if s.walletUpgradeRepo == nil {
		return nil, fmt.Errorf("wallet upgrade repository not configured")
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid upgrade request id")
	}
	return s.walletUpgradeRepo.GetWalletUpgradeRequestByID(ctx, uid)
}

// WalletUpgradeStatusResult is the result of GetWalletUpgradeStatus (user or admin). UpgradeStatus is from 9PSB upgrade_status API (source of truth); Latest is our most recent upgrade request row for reference.
type WalletUpgradeStatusResult struct {
	HasWallet     bool                              // user has an active wallet (we could call 9PSB)
	UpgradeStatus *psb.WaasUpgradeStatusResponse    // from 9PSB GET upgrade_status; nil if no wallet or 9PSB not configured
	Latest        *repository.WalletUpgradeRequestRow // our DB row for reference (optional)
}

// GetWalletUpgradeStatus returns wallet upgrade status from 9PSB upgrade_status API for the user's wallet. Used by user GET /wallet/upgrade-status and admin GET /users/:id/wallet/upgrade-status.
func (s *PaymentService) GetWalletUpgradeStatus(ctx context.Context, userID string) (*WalletUpgradeStatusResult, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user_id")
	}
	wallet, err := s.walletRepo.GetActiveByUserID(ctx, uid)
	if err != nil {
		return nil, err
	}
	result := &WalletUpgradeStatusResult{HasWallet: wallet != nil}
	if wallet == nil {
		return result, nil
	}
	// Primary: 9PSB upgrade_status (source of truth)
	if s.psbProvider != nil {
		upgradeResp, err := s.psbProvider.WaasUpgradeStatus(ctx, wallet.AccountNumber)
		if err != nil {
			// Network/auth/parse error; still return wallet + optional latest from DB
			result.UpgradeStatus = &psb.WaasUpgradeStatusResponse{
				Status:  "FAILED",
				Message: err.Error(),
			}
			result.UpgradeStatus.Data.Message = err.Error()
			result.UpgradeStatus.Data.Status = "Failed"
		} else {
			result.UpgradeStatus = upgradeResp
		}
	}
	// Optional: our latest upgrade request row for reference
	if s.walletUpgradeRepo != nil {
		latest, _ := s.walletUpgradeRepo.GetLatestByUserID(ctx, uid)
		result.Latest = latest
	}
	return result, nil
}

// ProcessWalletUpgradeWebhook processes a 9PSB WALLET_UPGRADE webhook: inserts webhook_events row, finds the matching upgrade request (SUBMITTED, final_status NULL), finalizes it (final_status, declined_reason, webhook_event_id, finalized_at) and on APPROVED updates wallets.tier in one transaction, then marks webhook PROCESSED.
func (s *PaymentService) ProcessWalletUpgradeWebhook(ctx context.Context, accountNumberHash, finalStatus, declinedReason string, rawPayload []byte) error {
	if s.webhookEventsRepo == nil || s.walletUpgradeRepo == nil {
		return fmt.Errorf("webhook or wallet upgrade repository not configured")
	}
	webhookID, err := s.webhookEventsRepo.InsertWebhookEvent(ctx, "WALLET_UPGRADE", finalStatus, "", accountNumberHash, rawPayload)
	if err != nil {
		return fmt.Errorf("insert webhook event: %w", err)
	}
	pending, err := s.walletUpgradeRepo.FindLatestSubmittedByAccountHash(ctx, accountNumberHash)
	if err != nil {
		return fmt.Errorf("find upgrade request: %w", err)
	}
	if pending == nil {
		_ = s.webhookEventsRepo.MarkProcessed(ctx, webhookID)
		return nil
	}
	webhookIDPtr := &webhookID
	if err := s.walletUpgradeRepo.FinalizeUpgrade(ctx, pending.ID, finalStatus, declinedReason, webhookIDPtr, pending.WalletID, pending.TierTo); err != nil {
		return fmt.Errorf("finalize upgrade: %w", err)
	}
	if err := s.webhookEventsRepo.MarkProcessed(ctx, webhookID); err != nil {
		return fmt.Errorf("mark webhook processed: %w", err)
	}
	return nil
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

// WalletDebitCreditResult is the result of an internal debit or credit.
type WalletDebitCreditResult struct {
	TransactionRef string
}

// WalletDebitCredit performs an internal debit or credit on the user's wallet (e.g. airtime, data, electricity, DSTV, admin adjust).
// Calls 9PSB WaaS debit/credit API first; updates our transactions and ledger only when 9PSB returns success.
func (s *PaymentService) WalletDebitCredit(ctx context.Context, userID string, amount float64, isCredit bool, narration string, initiatedBy string) (*WalletDebitCreditResult, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	if narration == "" {
		return nil, fmt.Errorf("narration is required")
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
	txnRef := generateTrackingRef("ADJ")
	// 1) Call 9PSB WaaS debit or credit; do not update our ledger until 9PSB approves
	if s.psbProvider != nil {
		var providerRef string
		if isCredit {
			providerRef, err = s.psbProvider.WaasCreditTransfer(ctx, wallet.AccountNumber, narration, amount, txnRef)
		} else {
			providerRef, err = s.psbProvider.WaasDebitTransfer(ctx, wallet.AccountNumber, narration, amount, txnRef)
		}
		if err != nil {
			if strings.Contains(err.Error(), "Duplicate") {
				return nil, fmt.Errorf("duplicate transaction reference: %w", err)
			}
			return nil, fmt.Errorf("9PSB: %w", err)
		}
		// 2) 9PSB success: update our transactions and ledger
		txnType := "DEBIT"
		direction := "OUT"
		if isCredit {
			txnType = "CREDIT"
			direction = "IN"
		}
		params := &repository.CreateInternalDebitCreditParams{
			WalletID:       wallet.WalletID,
			TransactionRef: txnRef,
			Type:           txnType,
			Direction:      direction,
			Amount:         amount,
			Narration:      narration,
			InitiatedBy:    initiatedBy,
			ProviderRef:    providerRef,
		}
		_, err = s.transactionRepo.CreateInternalDebitCreditAndPostLedger(ctx, params)
		if err != nil {
			if strings.Contains(err.Error(), "Insufficient balance") {
				return nil, fmt.Errorf("insufficient balance: %w", err)
			}
			return nil, err
		}
	} else {
		// No 9PSB WaaS configured: legacy local-only flow (for backward compatibility or tests)
		txnType := "DEBIT"
		direction := "OUT"
		if isCredit {
			txnType = "CREDIT"
			direction = "IN"
		}
		params := &repository.CreateInternalDebitCreditParams{
			WalletID:       wallet.WalletID,
			TransactionRef: txnRef,
			Type:           txnType,
			Direction:      direction,
			Amount:         amount,
			Narration:      narration,
			InitiatedBy:    initiatedBy,
		}
		_, err = s.transactionRepo.CreateInternalDebitCreditAndPostLedger(ctx, params)
		if err != nil {
			if strings.Contains(err.Error(), "Insufficient balance") {
				return nil, fmt.Errorf("insufficient balance: %w", err)
			}
			return nil, err
		}
	}
	// Send email to user on successful debit or credit
	var toEmail string
	if s.userClient != nil {
		if u, _ := s.userClient.GetUserForKYC(ctx, userID); u != nil && u.Found {
			toEmail = u.Email
		}
	}
	if toEmail != "" && s.notifier != nil {
		evType := "wallet_debit"
		subject := "Your PayUp wallet was debited"
		html := buildWalletDebitEmailHTML(amount, narration, txnRef)
		if isCredit {
			evType = "wallet_credit"
			subject = "Your PayUp wallet was credited"
			html = buildWalletCreditEmailHTML(amount, narration, txnRef)
		}
		_ = s.SendNotification(kafka.NotificationEvent{
			Type:    evType,
			Channel: "email",
			Metadata: map[string]interface{}{
				"to":             toEmail,
				"subject":        subject,
				"html":           html,
				"amount":         amount,
				"narration":      narration,
				"transaction_ref": txnRef,
			},
		})
	}
	return &WalletDebitCreditResult{TransactionRef: txnRef}, nil
}

func buildWalletDebitEmailHTML(amount float64, narration, txnRef string) string {
	return `<p>Your PayUp wallet was debited.</p>` +
		`<p><strong>Amount:</strong> NGN ` + fmt.Sprintf("%.2f", amount) + `</p>` +
		`<p><strong>Narration:</strong> ` + html.EscapeString(narration) + `</p>` +
		`<p><strong>Reference:</strong> ` + html.EscapeString(txnRef) + `</p>` +
		`<p>Thank you for using PayUp.</p>`
}

func buildWalletCreditEmailHTML(amount float64, narration, txnRef string) string {
	return `<p>Your PayUp wallet was credited.</p>` +
		`<p><strong>Amount:</strong> NGN ` + fmt.Sprintf("%.2f", amount) + `</p>` +
		`<p><strong>Narration:</strong> ` + html.EscapeString(narration) + `</p>` +
		`<p><strong>Reference:</strong> ` + html.EscapeString(txnRef) + `</p>` +
		`<p>Thank you for using PayUp.</p>`
}
