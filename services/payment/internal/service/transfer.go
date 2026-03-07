package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/kafka"
	"github.com/abubakvr/payup-backend/services/payment/internal/psb"
	"github.com/abubakvr/payup-backend/services/payment/internal/repository"
	"github.com/google/uuid"
)

// TransferResult is the successful outcome of an other-bank transfer.
type TransferResult struct {
	TransactionRef string `json:"transaction_ref"`
	SessionID      string `json:"session_id,omitempty"`
}

// TransferToOtherBankParams are the inputs for an other-bank transfer.
type TransferToOtherBankParams struct {
	UserID                  string
	Amount                  float64
	BankCode                string
	BeneficiaryName         string
	BeneficiaryAccountNumber string
	Pin                     string
	IdempotencyKey          string
}

// TransferToOtherBank runs the full flow: validate user, enquiry, create txn, call 9PSB, post DEBIT, send email.
// Returns (result, nil) on success; (nil, error) on failure. On idempotency hit (existing success), returns existing result.
func (s *PaymentService) TransferToOtherBank(ctx context.Context, p *TransferToOtherBankParams) (*TransferResult, error) {
	if s.transactionRepo == nil || s.walletRepo == nil || s.psbProvider == nil {
		return nil, fmt.Errorf("transfer not configured")
	}

	uid, err := uuid.Parse(p.UserID)
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

	// Get current balance from 9PSB (wallet_enquiry) before proceeding
	enquiry, err := s.psbProvider.WalletEnquiry(ctx, wallet.AccountNumber)
	if err != nil {
		return nil, fmt.Errorf("could not verify balance: %w", err)
	}
	if enquiry.AvailableBalance < p.Amount {
		return nil, fmt.Errorf("insufficient balance")
	}

	// 1) User validation (PIN, restricted, paused, limits)
	if s.userClient != nil {
		resp, err := s.userClient.ValidateTransfer(ctx, p.UserID, p.Amount, p.Pin)
		if err != nil {
			return nil, fmt.Errorf("validate transfer: %w", err)
		}
		if resp != nil && !resp.Allowed {
			return nil, fmt.Errorf("%s", resp.Message)
		}
		if resp != nil && (resp.DailyTransferLimit > 0 || resp.MonthlyTransferLimit > 0) {
			now := time.Now().UTC()
			todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
			monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
			until := now.Format(time.RFC3339)
			dailySpend, _ := s.transactionRepo.SumSuccessfulOutboundAmountByWalletAndWindow(ctx, wallet.WalletID, todayStart, until)
			monthlySpend, _ := s.transactionRepo.SumSuccessfulOutboundAmountByWalletAndWindow(ctx, wallet.WalletID, monthStart, until)
			if resp.DailyTransferLimit > 0 && (p.Amount+dailySpend) > resp.DailyTransferLimit {
				return nil, fmt.Errorf("daily transfer limit exceeded")
			}
			if resp.MonthlyTransferLimit > 0 && (p.Amount+monthlySpend) > resp.MonthlyTransferLimit {
				return nil, fmt.Errorf("monthly transfer limit exceeded")
			}
		}
	}

	// 2) Idempotency: if key provided and we already have a SUCCESS row, return it
	if p.IdempotencyKey != "" {
		existingID, existingStatus, err := s.transactionRepo.GetByIdempotencyKey(ctx, p.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existingID != uuid.Nil && existingStatus == "SUCCESS" {
			ref, provRef, _ := s.transactionRepo.GetRefAndProviderRefByID(ctx, existingID)
			return &TransferResult{TransactionRef: ref, SessionID: provRef}, nil
		}
	}

	// 3) Beneficiary name enquiry: 9PSB (120001) use wallet_enquiry; other banks use other_banks_enquiry
	const psbBankCode = "120001"
	var enquiryName string
	if p.BankCode == psbBankCode {
		walletEnq, err := s.psbProvider.WalletEnquiry(ctx, p.BeneficiaryAccountNumber)
		if err != nil {
			return nil, fmt.Errorf("beneficiary enquiry: %w", err)
		}
		enquiryName = walletEnq.Name
	} else {
		var err error
		enquiryName, err = s.psbProvider.OtherBanksEnquiry(ctx, p.BankCode, p.BeneficiaryAccountNumber)
		if err != nil {
			return nil, fmt.Errorf("beneficiary enquiry: %w", err)
		}
	}
	// Require match (case-insensitive trim)
	if strings.TrimSpace(strings.ToLower(enquiryName)) != strings.TrimSpace(strings.ToLower(p.BeneficiaryName)) {
		return nil, fmt.Errorf("beneficiary name does not match account; expected %q", enquiryName)
	}

	// 4) Generate ref and build 9PSB payload
	txnRef := generateTrackingRef("TXN")
	narration := fmt.Sprintf("Transfer to %s", p.BeneficiaryName)
	payload := &psb.WalletOtherBanksPayload{}
	payload.Customer.Account.Bank = p.BankCode
	payload.Customer.Account.Name = enquiryName
	payload.Customer.Account.Number = p.BeneficiaryAccountNumber
	payload.Customer.Account.SenderAccountNumber = wallet.AccountNumber
	payload.Customer.Account.SenderName = wallet.FullName
	payload.Narration = narration
	payload.Order.Amount = formatAmount(p.Amount)
	payload.Order.Country = "NGA"
	payload.Order.Currency = "NGN"
	payload.Order.Description = narration
	payload.Transaction.Reference = txnRef
	payload.Merchant.IsFee = false
	payload.Merchant.MerchantFeeAccount = ""
	payload.Merchant.MerchantFeeAmount = ""

	psbReqJSON, _ := json.Marshal(payload)

	// 5) Insert PENDING transaction (with idempotency if provided)
	createParams := &repository.CreateTransferParams{
		WalletID:              wallet.WalletID,
		TransactionRef:        txnRef,
		Amount:                p.Amount,
		FeeAmount:             0,
		FeeAccount:            "",
		Narration:             narration,
		BeneficiaryBank:       p.BankCode,
		BeneficiaryAcct:      p.BeneficiaryAccountNumber,
		BeneficiaryName:      enquiryName,
		SenderAccount:        wallet.AccountNumber,
		IdempotencyKey:       p.IdempotencyKey,
		InitiatedBy:          p.UserID,
		PsbRequestJSON:        psbReqJSON,
	}
	txnID, existingStatus, created, err := s.transactionRepo.CreateTransferWithIdempotency(ctx, createParams)
	if err != nil {
		return nil, err
	}
	if !created && existingStatus == "SUCCESS" {
		ref, provRef, _ := s.transactionRepo.GetRefAndProviderRefByID(ctx, txnID)
		return &TransferResult{TransactionRef: ref, SessionID: provRef}, nil
	}
	if !created {
		// Another request is in progress with same idempotency key; treat as conflict or wait
		return nil, fmt.Errorf("duplicate request; try again later")
	}

	// 6) Call 9PSB wallet_other_banks
	rawResp, sessionID, responseCode, err := s.psbProvider.WalletOtherBanks(ctx, payload)
	if err != nil {
		_ = s.transactionRepo.UpdateTransferAfterAPI(ctx, txnID, "FAILED", "", rawResp, responseCode)
		return nil, err
	}

	// 7) Update transaction SUCCESS
	if err := s.transactionRepo.UpdateTransferAfterAPI(ctx, txnID, "SUCCESS", sessionID, rawResp, responseCode); err != nil {
		return nil, err
	}

	// 8) Sync wallet from 9PSB (post-debit) then post DEBIT ledger entry so local balance matches provider
	enquiryPost, err := s.psbProvider.WalletEnquiry(ctx, wallet.AccountNumber)
	if err != nil {
		return nil, fmt.Errorf("sync balance after transfer: %w", err)
	}
	_, err = s.transactionRepo.PostLedgerEntryAfterSync(ctx, txnID, wallet.WalletID, p.Amount, narration,
		enquiryPost.AvailableBalance, enquiryPost.LedgerBalance)
	if err != nil {
		return nil, fmt.Errorf("ledger entry: %w", err)
	}

	// 9) Success email
	var toEmail string
	if s.userClient != nil {
		if u, _ := s.userClient.GetUserForKYC(ctx, p.UserID); u != nil && u.Found {
			toEmail = u.Email
		}
	}
	if toEmail != "" {
		_ = s.SendNotification(kafka.NotificationEvent{
			Type:    "transfer_success",
			Channel: "email",
			Metadata: map[string]interface{}{
				"to":       toEmail,
				"subject":  "Transfer successful",
				"html":     buildTransferSuccessEmailHTML(p.Amount, p.BeneficiaryName, txnRef),
				"amount":   p.Amount,
				"beneficiary": p.BeneficiaryName,
				"transaction_ref": txnRef,
			},
		})
	}
	_ = s.SendAuditLog(kafka.AuditLogParams{
		Action:   "transfer_success",
		Entity:   "transaction",
		EntityID: txnID.String(), // audit_logs.entity_id is UUID
		UserID:   &p.UserID,
		Metadata: map[string]interface{}{
			"amount": p.Amount, "beneficiary": p.BeneficiaryName,
			"transaction_ref": txnRef, "provider_ref": sessionID,
		},
	})

	return &TransferResult{TransactionRef: txnRef, SessionID: sessionID}, nil
}

func formatAmount(amount float64) string {
	if amount == float64(int64(amount)) {
		return fmt.Sprintf("%.0f", amount)
	}
	return fmt.Sprintf("%.2f", amount)
}

func buildTransferSuccessEmailHTML(amount float64, beneficiary, txnRef string) string {
	return `<p>Your transfer was successful.</p>` +
		`<p><strong>Amount:</strong> NGN ` + fmt.Sprintf("%.2f", amount) + `</p>` +
		`<p><strong>Beneficiary:</strong> ` + beneficiary + `</p>` +
		`<p><strong>Reference:</strong> ` + txnRef + `</p>` +
		`<p>Thank you for using PayUp.</p>`
}
