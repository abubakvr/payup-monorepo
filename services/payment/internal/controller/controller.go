package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abubakvr/payup-backend/services/payment/internal/auth"
	"github.com/abubakvr/payup-backend/services/payment/internal/config"
	"github.com/abubakvr/payup-backend/services/payment/internal/idempotency"
	"github.com/abubakvr/payup-backend/services/payment/internal/service"
	"github.com/abubakvr/payup-backend/services/payment/internal/validator"
	"github.com/gin-gonic/gin"
)

const idempotencyTTL = 24 * time.Hour

// PaymentController is the HTTP controller for the payment service.
type PaymentController struct {
	svc    *service.PaymentService
	cfg    *config.Config
	idem   *idempotency.Store
}

// NewPaymentController returns a new controller.
func NewPaymentController(svc *service.PaymentService, cfg *config.Config) *PaymentController {
	idem := idempotency.NewStore(cfg.RedisAddr, cfg.RedisPassword, idempotencyTTL)
	return &PaymentController{svc: svc, cfg: cfg, idem: idem}
}

// Health returns 200 if the service and DB are healthy.
func (c *PaymentController) Health(ctx *gin.Context) {
	if err := c.svc.Health(ctx.Request.Context()); err != nil {
		Error(ctx, http.StatusServiceUnavailable, err.Error(), CodeInternal)
		return
	}
	Success(ctx, http.StatusOK, "ok", CodeSuccess, gin.H{"status": "ok"})
}

// OpenWalletRequest is the JSON body for POST /wallets.
type OpenWalletRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// OpenWallet creates a 9PSB wallet for the user (KYC via gRPC, then 9PSB open_wallet; store only on success).
func (c *PaymentController) OpenWallet(ctx *gin.Context) {
	var body OpenWalletRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		Error(ctx, http.StatusBadRequest, "user_id required", CodeBadRequest)
		return
	}
	if err := validator.ValidateUserID(body.UserID); err != nil {
		Error(ctx, http.StatusBadRequest, err.Error(), CodeBadRequest)
		return
	}
	accountNumber, err := c.svc.CreateWallet(ctx.Request.Context(), body.UserID)
	if err != nil {
		if errors.Is(err, service.ErrActiveWalletExists) {
			Error(ctx, http.StatusConflict, err.Error(), CodeConflict)
			return
		}
		if errors.Is(err, service.ErrKYCNotFound) {
			Error(ctx, http.StatusPreconditionFailed, err.Error(), CodeConflict)
			return
		}
		var valErr *validator.ValidationErrors
		if errors.As(err, &valErr) {
			Error(ctx, http.StatusBadRequest, valErr.Error(), CodeBadRequest)
			return
		}
		if errors.Is(err, validator.ErrValidation) || strings.Contains(err.Error(), "validation") {
			Error(ctx, http.StatusBadRequest, err.Error(), CodeBadRequest)
			return
		}
		Error(ctx, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	Success(ctx, http.StatusCreated, "Wallet created", CodeSuccess, gin.H{"account_number": accountNumber})
}

// GetWallet returns the authenticated user's wallet details from the database (account number, account name, status). Requires JWT.
func (c *PaymentController) GetWallet(ctx *gin.Context) {
	userID, err := auth.DecodeUserIDFromRequest(ctx.GetHeader("Authorization"), c.cfg.JWTSecret)
	if err != nil {
		AbortUnauthorized(ctx, "invalid or missing token")
		return
	}
	details, err := c.svc.GetWalletDetails(ctx.Request.Context(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "no active wallet") {
			Error(ctx, http.StatusNotFound, err.Error(), CodeConflict)
			return
		}
		if strings.Contains(err.Error(), "invalid user_id") {
			Error(ctx, http.StatusBadRequest, err.Error(), CodeBadRequest)
			return
		}
		Error(ctx, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	Success(ctx, http.StatusOK, "Successful", CodeSuccess, gin.H{
		"account_number": details.AccountNumber,
		"account_name":   details.AccountName,
		"status":         details.Status,
	})
}

// GetWalletTransactions returns the authenticated user's wallet transaction history. Query: limit (default 20, max 100), offset (default 0). Requires JWT.
func (c *PaymentController) GetWalletTransactions(ctx *gin.Context) {
	userID, err := auth.DecodeUserIDFromRequest(ctx.GetHeader("Authorization"), c.cfg.JWTSecret)
	if err != nil {
		AbortUnauthorized(ctx, "invalid or missing token")
		return
	}
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(ctx.DefaultQuery("offset", "0"))
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	list, err := c.svc.GetWalletTransactionHistory(ctx.Request.Context(), userID, limit, offset)
	if err != nil {
		if strings.Contains(err.Error(), "no active wallet") {
			Error(ctx, http.StatusNotFound, err.Error(), CodeConflict)
			return
		}
		if strings.Contains(err.Error(), "invalid user_id") {
			Error(ctx, http.StatusBadRequest, err.Error(), CodeBadRequest)
			return
		}
		Error(ctx, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	transactions := make([]gin.H, 0, len(list))
	for _, row := range list {
		transactions = append(transactions, gin.H{
			"transaction_ref":   row.TransactionRef,
			"type":              row.Type,
			"direction":         row.Direction,
			"amount":            row.Amount,
			"fee_amount":        row.FeeAmount,
			"narration":         row.Narration,
			"status":            row.Status,
			"channel":           row.Channel,
			"beneficiary_bank":  row.BeneficiaryBank,
			"beneficiary_name":  row.BeneficiaryName,
			"created_at":        row.CreatedAt.Format(time.RFC3339),
		})
	}
	Success(ctx, http.StatusOK, "Successful", CodeSuccess, gin.H{"transactions": transactions})
}

// GetTransactionDetail returns a single transaction by transaction_ref for the authenticated user's wallet. Requires JWT.
func (c *PaymentController) GetTransactionDetail(ctx *gin.Context) {
	userID, err := auth.DecodeUserIDFromRequest(ctx.GetHeader("Authorization"), c.cfg.JWTSecret)
	if err != nil {
		AbortUnauthorized(ctx, "invalid or missing token")
		return
	}
	transactionRef := ctx.Param("transaction_ref")
	if transactionRef == "" {
		Error(ctx, http.StatusBadRequest, "transaction_ref is required", CodeBadRequest)
		return
	}
	row, err := c.svc.GetTransactionDetail(ctx.Request.Context(), userID, transactionRef)
	if err != nil {
		if strings.Contains(err.Error(), "no active wallet") {
			Error(ctx, http.StatusNotFound, err.Error(), CodeConflict)
			return
		}
		if strings.Contains(err.Error(), "invalid user_id") {
			Error(ctx, http.StatusBadRequest, err.Error(), CodeBadRequest)
			return
		}
		Error(ctx, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	if row == nil {
		Error(ctx, http.StatusNotFound, "transaction not found", CodeConflict)
		return
	}
	Success(ctx, http.StatusOK, "Successful", CodeSuccess, gin.H{
		"transaction_ref":   row.TransactionRef,
		"type":             row.Type,
		"direction":        row.Direction,
		"amount":           row.Amount,
		"fee_amount":       row.FeeAmount,
		"narration":        row.Narration,
		"status":           row.Status,
		"channel":          row.Channel,
		"beneficiary_bank": row.BeneficiaryBank,
		"beneficiary_name": row.BeneficiaryName,
		"created_at":       row.CreatedAt.Format(time.RFC3339),
	})
}

// GetBalance returns the authenticated user's wallet balance (live from 9PSB wallet_enquiry). Requires JWT.
func (c *PaymentController) GetBalance(ctx *gin.Context) {
	userID, err := auth.DecodeUserIDFromRequest(ctx.GetHeader("Authorization"), c.cfg.JWTSecret)
	if err != nil {
		AbortUnauthorized(ctx, "invalid or missing token")
		return
	}
	result, err := c.svc.GetWalletBalance(ctx.Request.Context(), userID)
	if err != nil {
		if strings.Contains(err.Error(), "no active wallet") {
			Error(ctx, http.StatusNotFound, err.Error(), CodeConflict)
			return
		}
		if strings.Contains(err.Error(), "invalid user_id") {
			Error(ctx, http.StatusBadRequest, err.Error(), CodeBadRequest)
			return
		}
		Error(ctx, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	data := gin.H{
		"available_balance": result.AvailableBalance,
		"ledger_balance":    result.LedgerBalance,
		"account_number":    result.Nuban,
		"name":              result.Name,
		"status":            result.Status,
	}
	Success(ctx, http.StatusOK, "Successful", CodeSuccess, data)
}

// BeneficiaryEnquiryRequest is the JSON body for POST /wallet/beneficiary-enquiry (resolve account name for display).
type BeneficiaryEnquiryRequest struct {
	BankCode      string `json:"bank_code" binding:"required"`
	AccountNumber string `json:"account_number" binding:"required"`
}

// BeneficiaryEnquiry resolves beneficiary name: 9PSB (120001) uses wallet_enquiry, other banks use other_banks_enquiry. Requires JWT.
func (c *PaymentController) BeneficiaryEnquiry(ctx *gin.Context) {
	_, err := auth.DecodeUserIDFromRequest(ctx.GetHeader("Authorization"), c.cfg.JWTSecret)
	if err != nil {
		AbortUnauthorized(ctx, "invalid or missing token")
		return
	}
	var body BeneficiaryEnquiryRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		Error(ctx, http.StatusBadRequest, "bank_code and account_number required", CodeBadRequest)
		return
	}
	result, err := c.svc.ResolveBeneficiary(ctx.Request.Context(), body.BankCode, body.AccountNumber)
	if err != nil {
		if strings.Contains(err.Error(), "account not found") || strings.Contains(err.Error(), "Invalid") {
			Error(ctx, http.StatusBadRequest, err.Error(), CodeBadRequest)
			return
		}
		Error(ctx, http.StatusInternalServerError, err.Error(), CodeInternal)
		return
	}
	data := gin.H{
		"name":           result.Name,
		"account_number": result.AccountNumber,
		"bank_code":      result.BankCode,
	}
	if result.AvailableBalance > 0 {
		data["available_balance"] = result.AvailableBalance
	}
	Success(ctx, http.StatusOK, "Successful", CodeSuccess, data)
}

// TransferRequest is the JSON body for POST /transfers (other-bank transfer).
type TransferRequest struct {
	Amount                    float64 `json:"amount" binding:"required,gt=0"`
	BankCode                  string  `json:"bank_code" binding:"required"`
	BeneficiaryName           string  `json:"beneficiary_name" binding:"required"`
	BeneficiaryAccountNumber string  `json:"beneficiary_account_number" binding:"required"`
	Pin                       string  `json:"pin" binding:"required,len=4"`
}

// TransferToOtherBank handles POST /transfers. Requires JWT; optional X-Idempotency-Key.
func (c *PaymentController) TransferToOtherBank(ctx *gin.Context) {
	userID, err := auth.DecodeUserIDFromRequest(ctx.GetHeader("Authorization"), c.cfg.JWTSecret)
	if err != nil {
		AbortUnauthorized(ctx, "invalid or missing token")
		return
	}
	idempotencyKey := strings.TrimSpace(ctx.GetHeader("X-Idempotency-Key"))
	if idempotencyKey == "" {
		Error(ctx, http.StatusBadRequest, "X-Idempotency-Key header is required for transfer requests", CodeBadRequest)
		return
	}
	if cached, ok := c.idem.Get(ctx.Request.Context(), idempotencyKey); ok && len(cached) > 0 {
		ctx.Data(http.StatusOK, "application/json", cached)
		return
	}
	var body TransferRequest
	if err := ctx.ShouldBindJSON(&body); err != nil {
		Error(ctx, http.StatusBadRequest, "invalid body: amount, bank_code, beneficiary_name, beneficiary_account_number, pin (4 digits) required", CodeBadRequest)
		return
	}
	if len(body.Pin) != 4 {
		Error(ctx, http.StatusBadRequest, "pin must be exactly 4 digits", CodeBadRequest)
		return
	}
	result, err := c.svc.TransferToOtherBank(ctx.Request.Context(), &service.TransferToOtherBankParams{
		UserID:                   userID,
		Amount:                   body.Amount,
		BankCode:                 body.BankCode,
		BeneficiaryName:          body.BeneficiaryName,
		BeneficiaryAccountNumber: body.BeneficiaryAccountNumber,
		Pin:                      body.Pin,
		IdempotencyKey:           idempotencyKey,
	})
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "insufficient balance") {
			Error(ctx, http.StatusBadRequest, msg, CodeBadRequest)
			return
		}
		if strings.Contains(msg, "invalid PIN") || strings.Contains(msg, "PIN not set") || strings.Contains(msg, "PIN required") {
			Error(ctx, http.StatusUnauthorized, msg, CodeUnauthorized)
			return
		}
		if strings.Contains(msg, "account restricted") || strings.Contains(msg, "transfers paused") {
			Error(ctx, http.StatusForbidden, msg, CodeForbidden)
			return
		}
		if strings.Contains(msg, "daily transfer limit") || strings.Contains(msg, "monthly transfer limit") {
			Error(ctx, http.StatusBadRequest, msg, CodeBadRequest)
			return
		}
		if strings.Contains(msg, "beneficiary name does not match") {
			Error(ctx, http.StatusBadRequest, msg, CodeBadRequest)
			return
		}
		Error(ctx, http.StatusInternalServerError, msg, CodeInternal)
		return
	}
	data := gin.H{"transaction_ref": result.TransactionRef, "session_id": result.SessionID}
	resp := ApiResponse{Status: "success", Message: "Transfer successful", ResponseCode: CodeSuccess, Data: data}
	bodyBytes, _ := json.Marshal(resp)
	c.idem.Set(ctx.Request.Context(), idempotencyKey, bodyBytes)
	ctx.JSON(http.StatusCreated, resp)
}
