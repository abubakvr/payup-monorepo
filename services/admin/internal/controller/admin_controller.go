package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/abubakvr/payup-backend/services/admin/internal/auth"
	"github.com/abubakvr/payup-backend/services/admin/internal/clients"
	"github.com/abubakvr/payup-backend/services/admin/internal/dto"
	"github.com/abubakvr/payup-backend/services/admin/internal/kafka"
	"github.com/abubakvr/payup-backend/services/admin/internal/model"
	"github.com/abubakvr/payup-backend/services/admin/internal/repository"
	"github.com/abubakvr/payup-backend/services/admin/internal/service"
	"github.com/gin-gonic/gin"
)

type AdminController struct {
	svc                  *service.AdminService
	user                 *clients.UserAdminClient
	kyc                  *clients.KYCAdminClient
	audit                *clients.AuditAdminClient
	payment              *clients.PaymentAdminClient
	auditProducer        *kafka.AuditProducer
	notificationProducer *kafka.NotificationProducer
	portalURL            string
	kycKey               string
}

func NewAdminController(svc *service.AdminService, user *clients.UserAdminClient, kyc *clients.KYCAdminClient, audit *clients.AuditAdminClient, payment *clients.PaymentAdminClient, auditProducer *kafka.AuditProducer, notificationProducer *kafka.NotificationProducer, portalURL, kycAdminKey string) *AdminController {
	return &AdminController{svc: svc, user: user, kyc: kyc, audit: audit, payment: payment, auditProducer: auditProducer, notificationProducer: notificationProducer, portalURL: portalURL, kycKey: kycAdminKey}
}

// respondSuccess sends 200 with common ApiResponse envelope (status success, responseCode 01).
func respondSuccess(ctx *gin.Context, message string, data interface{}) {
	ctx.JSON(http.StatusOK, dto.ApiResponse{
		Data:         data,
		ResponseCode: "01",
		Status:       "success",
		Message:      message,
	})
}

// respondCreated sends 201 with common ApiResponse envelope.
func respondCreated(ctx *gin.Context, message string, data interface{}) {
	ctx.JSON(http.StatusCreated, dto.ApiResponse{
		Data:         data,
		ResponseCode: "01",
		Status:       "success",
		Message:      message,
	})
}

// respondError sends statusCode with common ApiResponse envelope (status error, data nil).
func respondError(ctx *gin.Context, statusCode int, responseCode, message string) {
	ctx.JSON(statusCode, dto.ApiResponse{
		Data:         nil,
		ResponseCode: responseCode,
		Status:       "error",
		Message:      message,
	})
}

// Login POST /auth/login
func (c *AdminController) Login(ctx *gin.Context) {
	clientIP := ctx.ClientIP()
	userAgent := strings.TrimSpace(ctx.GetHeader("User-Agent"))
	if userAgent == "" {
		userAgent = "-"
	}
	var req dto.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Printf("admin login failed email=%s ip=%s device=%s reason=invalid_request", req.Email, clientIP, userAgent)
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_login_failed", "admin", "", "", map[string]interface{}{"email": req.Email, "reason": "invalid_request"})
		}
		respondError(ctx, http.StatusBadRequest, "02", "invalid request")
		return
	}
	token, expiresAt, mustChange, adminID, err := c.svc.Login(ctx.Request.Context(), req.Email, req.Password)
	if err != nil {
		reason := "invalid_credentials"
		if err != service.ErrInvalidCredentials {
			reason = err.Error()
		}
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_login_failed", "admin", "", "", map[string]interface{}{"email": req.Email, "reason": reason})
		}
		if err == service.ErrInvalidCredentials {
			log.Printf("admin login failed email=%s ip=%s device=%s reason=invalid_credentials", req.Email, clientIP, userAgent)
			respondError(ctx, http.StatusUnauthorized, "AUTH_401", "invalid email or password")
			return
		}
		log.Printf("admin login failed email=%s ip=%s device=%s err=%v", req.Email, clientIP, userAgent, err)
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if c.auditProducer != nil {
		_ = c.auditProducer.SendAudit("admin_login_success", "admin", adminID, adminID, map[string]interface{}{"email": req.Email})
	}
	log.Printf("admin login success email=%s ip=%s device=%s", req.Email, clientIP, userAgent)
	respondSuccess(ctx, "Login successful", dto.LoginResponse{
		AccessToken:        token,
		ExpiresAt:           expiresAt,
		MustChangePassword: mustChange,
	})
}

// ChangePassword POST /auth/change-password (requires admin JWT)
func (c *AdminController) ChangePassword(ctx *gin.Context) {
	claims, ok := auth.ClaimsFrom(ctx)
	if !ok {
		respondError(ctx, http.StatusUnauthorized, "AUTH_401", "unauthorized")
		return
	}
	var req dto.ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondError(ctx, http.StatusBadRequest, "02", "invalid request")
		return
	}
	err := c.svc.ChangePassword(ctx.Request.Context(), claims.AdminID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		if err == service.ErrInvalidCredentials {
			respondError(ctx, http.StatusBadRequest, "02", "current password is incorrect")
			return
		}
		if err == repository.ErrAdminNotFound {
			respondError(ctx, http.StatusUnauthorized, "AUTH_401", "unauthorized")
			return
		}
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	respondSuccess(ctx, "password updated", nil)
}

// CreateAdmin POST /admins (super_admin only)
func (c *AdminController) CreateAdmin(ctx *gin.Context) {
	claims, ok := auth.ClaimsFrom(ctx)
	if !ok {
		respondError(ctx, http.StatusUnauthorized, "AUTH_401", "unauthorized")
		return
	}
	if claims.Role != model.RoleSuperAdmin {
		respondError(ctx, http.StatusForbidden, "99", "only super admin can create admins")
		return
	}
	var req dto.CreateAdminRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondError(ctx, http.StatusBadRequest, "02", "invalid request")
		return
	}
	admin, err := c.svc.CreateAdmin(ctx.Request.Context(), claims.AdminID, req.Email, req.Phone, req.FirstName, req.LastName, req.TemporaryPassword)
	if err != nil {
		if err == repository.ErrEmailExists {
			respondError(ctx, http.StatusConflict, "99", "admin with this email already exists")
			return
		}
		if err == service.ErrOnlySuperAdminCreate {
			respondError(ctx, http.StatusForbidden, "99", err.Error())
			return
		}
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	respondCreated(ctx, "admin created", dto.AdminResponse{
		ID:                 admin.ID,
		Email:              admin.Email,
		Phone:              admin.Phone,
		FirstName:          admin.FirstName,
		LastName:           admin.LastName,
		Role:               admin.Role,
		MustChangePassword: admin.MustChangePassword,
	})
	// Audit log: admin_created (fire-and-forget)
	if c.auditProducer != nil {
		_ = c.auditProducer.SendAdminCreated(claims.AdminID, admin.ID, map[string]interface{}{
			"email":     admin.Email,
			"phone":     admin.Phone,
			"firstName": admin.FirstName,
			"lastName":  admin.LastName,
			"role":      admin.Role,
		})
	}
	// Send welcome email with login details and temporary password (fire-and-forget)
	if c.notificationProducer != nil {
		_ = c.notificationProducer.SendAdminWelcomeEmail(admin.Email, admin.FirstName, admin.LastName, req.TemporaryPassword, c.portalURL)
	}
}

// GetMe GET /me (current admin)
func (c *AdminController) GetMe(ctx *gin.Context) {
	claims, ok := auth.ClaimsFrom(ctx)
	if !ok {
		respondError(ctx, http.StatusUnauthorized, "AUTH_401", "unauthorized")
		return
	}
	admin, err := c.svc.GetMe(ctx.Request.Context(), claims.AdminID)
	if err != nil {
		if err == repository.ErrAdminNotFound {
			respondError(ctx, http.StatusUnauthorized, "AUTH_401", "unauthorized")
			return
		}
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	respondSuccess(ctx, "ok", dto.AdminResponse{
		ID:                 admin.ID,
		Email:              admin.Email,
		Phone:              admin.Phone,
		FirstName:          admin.FirstName,
		LastName:           admin.LastName,
		Role:               admin.Role,
		MustChangePassword: admin.MustChangePassword,
	})
}

// ListWallets GET /wallets (admin JWT) — list all user wallets with details via payment service gRPC (paginated: limit, offset).
func (c *AdminController) ListWallets(ctx *gin.Context) {
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	limit, offset := int32(50), int32(0)
	if l := ctx.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = int32(n)
			if limit > 100 {
				limit = 100
			}
		}
	}
	if o := ctx.Query("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	resp, err := c.payment.ListWallets(ctx.Request.Context(), limit, offset)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if resp == nil {
		respondSuccess(ctx, "ok", gin.H{"wallets": []interface{}{}, "limit": limit, "offset": offset})
		return
	}
	// Map proto WalletDetail to simple maps for JSON (account_number, user_id, etc.)
	wallets := make([]map[string]interface{}, 0, len(resp.Wallets))
	for _, w := range resp.Wallets {
		wallets = append(wallets, map[string]interface{}{
			"id":                w.Id,
			"user_id":           w.UserId,
			"account_number":    w.AccountNumber,
			"customer_id":       w.CustomerId,
			"order_ref":         w.OrderRef,
			"full_name":         w.FullName,
			"phone":             w.Phone,
			"email":             w.Email,
			"mfb_code":          w.MfbCode,
			"tier":              w.Tier,
			"status":            w.Status,
			"ledger_balance":    w.LedgerBalance,
			"available_balance": w.AvailableBalance,
			"provider":          w.Provider,
			"created_at":        w.CreatedAt,
			"updated_at":        w.UpdatedAt,
		})
	}
	respondSuccess(ctx, "ok", gin.H{"wallets": wallets, "limit": limit, "offset": offset})
}

// ListUsers GET /users (admin JWT) — list all users via user service gRPC (paginated: page, page_size or limit, offset)
func (c *AdminController) ListUsers(ctx *gin.Context) {
	if c.user == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "user service unavailable")
		return
	}
	page, pageSize := 1, 20
	if p := ctx.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if s := ctx.Query("page_size"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			pageSize = n
		}
	}
	limit := int32(pageSize)
	offset := int32((page - 1) * pageSize)
	if l := ctx.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			if n > 500 {
				n = 500
			}
			limit = int32(n)
		}
	}
	if o := ctx.Query("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	if (ctx.Query("limit") != "" || ctx.Query("offset") != "") && limit > 0 {
		page = int(offset/limit) + 1
		pageSize = int(limit)
	}
	resp, err := c.user.ListUsers(ctx.Request.Context(), limit, offset)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	respondSuccess(ctx, "ok", gin.H{
		"users":     resp.Users,
		"total":     resp.Total,
		"page":      page,
		"pageSize":  pageSize,
	})
}

// ListKYC GET /kyc-list (admin JWT) — list KYC summaries by joining user list with KYC service gRPC
func (c *AdminController) ListKYC(ctx *gin.Context) {
	if c.user == nil || c.kyc == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "user or kyc service unavailable")
		return
	}

	// Pagination: page & page_size (1-based page)
	page := 1
	pageSize := 20
	if p := ctx.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if s := ctx.Query("page_size"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			pageSize = n
		}
	}
	offset := int32((page - 1) * pageSize)
	limit := int32(pageSize)

	statusFilter := ctx.Query("status")    // e.g. approved, pending_review
	levelFilter := ctx.Query("kycLevel")   // numeric string

	// Total = count of KYC profiles (with same filters), not total users
	var kycTotal int64
	var levelPtr *int32
	if levelFilter != "" {
		if n, e := strconv.Atoi(levelFilter); e == nil {
			l := int32(n)
			levelPtr = &l
		}
	}
	kycTotal, err := c.kyc.CountProfiles(ctx.Request.Context(), statusFilter, levelPtr)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}

	userResp, err := c.user.ListUsers(ctx.Request.Context(), limit, offset)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}

	type KYCItem struct {
		UserID        string  `json:"userId"`
		Email         string  `json:"email"`
		FirstName     string  `json:"firstName"`
		LastName      string  `json:"lastName"`
		KYCLevel      int32   `json:"kycLevel"`
		OverallStatus string  `json:"overallStatus"`
		SubmittedAt   *string `json:"submittedAt,omitempty"`
	}

	items := make([]KYCItem, 0, len(userResp.Users))
	for _, u := range userResp.Users {
		if u == nil || u.Id == "" {
			continue
		}
		kycResp, err := c.kyc.GetFullKYCForAdmin(ctx.Request.Context(), u.Id)
		if err != nil || kycResp == nil || !kycResp.Found || kycResp.JsonPayload == "" {
			// Skip users without KYC data
			continue
		}
		var payload struct {
			Profile struct {
				KYCLevel      int32   `json:"kycLevel"`
				OverallStatus string  `json:"overallStatus"`
				SubmittedAt   *string `json:"submittedAt,omitempty"`
			} `json:"profile"`
		}
		if err := json.Unmarshal([]byte(kycResp.JsonPayload), &payload); err != nil {
			continue
		}

		// Apply optional filters
		if statusFilter != "" && payload.Profile.OverallStatus != statusFilter {
			continue
		}
		if levelFilter != "" {
			if n, err := strconv.Atoi(levelFilter); err == nil {
				if int32(n) != payload.Profile.KYCLevel {
					continue
				}
			}
		}

		items = append(items, KYCItem{
			UserID:        u.Id,
			Email:         u.Email,
			FirstName:     u.FirstName,
			LastName:      u.LastName,
			KYCLevel:      payload.Profile.KYCLevel,
			OverallStatus: payload.Profile.OverallStatus,
			SubmittedAt:   payload.Profile.SubmittedAt,
		})
	}

	respondSuccess(ctx, "ok", gin.H{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    kycTotal,
	})
}

// GetUser GET /users/:id (admin JWT) — single user via user service gRPC
func (c *AdminController) GetUser(ctx *gin.Context) {
	if c.user == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "user service unavailable")
		return
	}
	id := ctx.Param("id")
	if id == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	resp, err := c.user.GetUserForAdmin(ctx.Request.Context(), id)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Found {
		respondError(ctx, http.StatusNotFound, "02", "user not found")
		return
	}
	respondSuccess(ctx, "ok", resp.User)
}

// SetUserRestricted POST /users/:id/restrict (admin JWT) — body: {"restricted": true|false}. Calls user gRPC; user service audits and sends email when restricting.
func (c *AdminController) SetUserRestricted(ctx *gin.Context) {
	if c.user == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "user service unavailable")
		return
	}
	id := ctx.Param("id")
	if id == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	var body struct {
		Restricted bool `json:"restricted"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		respondError(ctx, http.StatusBadRequest, "02", "invalid body: need {\"restricted\": true|false}")
		return
	}
	resp, err := c.user.SetUserRestricted(ctx.Request.Context(), id, body.Restricted)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Success {
		if resp.Message == "user not found" {
			respondError(ctx, http.StatusNotFound, "02", resp.Message)
			return
		}
		respondError(ctx, http.StatusBadRequest, "02", resp.Message)
		return
	}
	respondSuccess(ctx, "ok", map[string]interface{}{"restricted": body.Restricted})
}

// CreateUserWallet POST /users/:id/wallet (admin JWT) — create 9PSB wallet for user. Calls payment service gRPC; payment fetches KYC via gRPC, calls 9PSB, saves wallet and emits audit + success email via Kafka.
func (c *AdminController) CreateUserWallet(ctx *gin.Context) {
	claims, _ := auth.ClaimsFrom(ctx)
	adminID := ""
	if claims != nil {
		adminID = claims.AdminID
	}
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	userID := ctx.Param("id")
	if userID == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	resp, err := c.payment.CreateWallet(ctx.Request.Context(), userID)
	if err != nil {
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_wallet_creation_failed", "wallet", userID, adminID, map[string]interface{}{"user_id": userID, "error": err.Error()})
		}
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Success {
		msg := resp.ErrorMessage
		if msg == "" {
			msg = "wallet creation failed"
		}
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_wallet_creation_failed", "wallet", userID, adminID, map[string]interface{}{"user_id": userID, "error": msg})
		}
		if strings.Contains(msg, "active wallet already exists") {
			respondError(ctx, http.StatusConflict, "02", msg)
			return
		}
		if strings.Contains(msg, "KYC not found") || strings.Contains(msg, "validation") {
			respondError(ctx, http.StatusPreconditionFailed, "02", msg)
			return
		}
		respondError(ctx, http.StatusBadRequest, "02", msg)
		return
	}
	if c.auditProducer != nil {
		_ = c.auditProducer.SendAudit("admin_wallet_created", "wallet", resp.AccountNumber, adminID, map[string]interface{}{"user_id": userID, "account_number": resp.AccountNumber})
	}
	respondCreated(ctx, "wallet created successfully", map[string]interface{}{
		"user_id":        userID,
		"account_number": resp.AccountNumber,
	})
}

// AdjustUserWallet POST /users/:id/wallet/adjust (admin JWT) — debit or credit user wallet (airtime, data, electricity, DSTV, etc.). Body: amount, type ("debit"|"credit"), narration.
func (c *AdminController) AdjustUserWallet(ctx *gin.Context) {
	claims, _ := auth.ClaimsFrom(ctx)
	adminID := ""
	if claims != nil {
		adminID = claims.AdminID
	}
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	userID := ctx.Param("id")
	if userID == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	var body struct {
		Amount    float64 `json:"amount" binding:"required,gt=0"`
		Type      string  `json:"type" binding:"required,oneof=debit credit"`
		Narration string  `json:"narration" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		respondError(ctx, http.StatusBadRequest, "02", "invalid body: amount (positive number), type (debit or credit), narration (required)")
		return
	}
	isCredit := body.Type == "credit"
	resp, err := c.payment.DebitCreditWallet(ctx.Request.Context(), userID, body.Amount, isCredit, body.Narration, adminID)
	if err != nil {
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_wallet_adjust_failed", "wallet", userID, adminID, map[string]interface{}{"user_id": userID, "amount": body.Amount, "type": body.Type, "error": err.Error()})
		}
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Success {
		msg := resp.ErrorMessage
		if msg == "" {
			msg = "wallet adjust failed"
		}
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_wallet_adjust_failed", "wallet", userID, adminID, map[string]interface{}{"user_id": userID, "amount": body.Amount, "type": body.Type, "error": msg})
		}
		if strings.Contains(msg, "no active wallet") {
			respondError(ctx, http.StatusNotFound, "02", msg)
			return
		}
		if strings.Contains(msg, "insufficient balance") {
			respondError(ctx, http.StatusBadRequest, "02", msg)
			return
		}
		respondError(ctx, http.StatusBadRequest, "02", msg)
		return
	}
	if c.auditProducer != nil {
		_ = c.auditProducer.SendAudit("admin_wallet_adjusted", "wallet", userID, adminID, map[string]interface{}{"user_id": userID, "amount": body.Amount, "type": body.Type, "narration": body.Narration, "transaction_ref": resp.TransactionRef})
	}
	respondSuccess(ctx, "wallet adjusted successfully", map[string]interface{}{
		"user_id":         userID,
		"amount":          body.Amount,
		"type":            body.Type,
		"narration":       body.Narration,
		"transaction_ref": resp.TransactionRef,
	})
}

// ChangeUserWalletStatus PUT /users/:id/wallet/status (admin JWT) — change user wallet status via 9PSB (ACTIVE or SUSPENDED). Body: { "status": "ACTIVE" | "SUSPENDED" }. Payment service sends audit + email.
func (c *AdminController) ChangeUserWalletStatus(ctx *gin.Context) {
	claims, _ := auth.ClaimsFrom(ctx)
	adminID := ""
	if claims != nil {
		adminID = claims.AdminID
	}
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	userID := ctx.Param("id")
	if userID == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	var body struct {
		Status string `json:"status" binding:"required,oneof=ACTIVE SUSPENDED"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		respondError(ctx, http.StatusBadRequest, "02", "invalid body: status (ACTIVE or SUSPENDED) required")
		return
	}
	resp, err := c.payment.ChangeWalletStatus(ctx.Request.Context(), userID, body.Status, adminID)
	if err != nil {
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_wallet_status_change_failed", "wallet", userID, adminID, map[string]interface{}{"user_id": userID, "status": body.Status, "error": err.Error()})
		}
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Success {
		msg := resp.ErrorMessage
		if msg == "" {
			msg = "wallet status change failed"
		}
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_wallet_status_change_failed", "wallet", userID, adminID, map[string]interface{}{"user_id": userID, "status": body.Status, "error": msg})
		}
		if strings.Contains(msg, "no wallet found") {
			respondError(ctx, http.StatusNotFound, "02", msg)
			return
		}
		respondError(ctx, http.StatusBadRequest, "02", msg)
		return
	}
	respondSuccess(ctx, "wallet status updated successfully", map[string]interface{}{
		"user_id":          userID,
		"new_wallet_status": resp.NewWalletStatus,
	})
}

// SubmitUserWalletUpgrade POST /users/:id/wallet/upgrade (admin JWT) — submit wallet tier upgrade to 9PSB. Payment fetches KYC + images via gRPC, sends multipart to 9PSB; audit + email via Kafka.
func (c *AdminController) SubmitUserWalletUpgrade(ctx *gin.Context) {
	claims, _ := auth.ClaimsFrom(ctx)
	adminID := ""
	if claims != nil {
		adminID = claims.AdminID
	}
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	userID := ctx.Param("id")
	if userID == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	resp, err := c.payment.SubmitWalletUpgrade(ctx.Request.Context(), userID, adminID)
	if err != nil {
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_wallet_upgrade_failed", "wallet", userID, adminID, map[string]interface{}{"user_id": userID, "error": err.Error()})
		}
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Success {
		msg := resp.ErrorMessage
		if msg == "" {
			msg = "wallet upgrade submission failed"
		}
		if c.auditProducer != nil {
			_ = c.auditProducer.SendAudit("admin_wallet_upgrade_failed", "wallet", userID, adminID, map[string]interface{}{"user_id": userID, "error": msg})
		}
		if strings.Contains(msg, "no active wallet") {
			respondError(ctx, http.StatusNotFound, "02", msg)
			return
		}
		respondError(ctx, http.StatusBadRequest, "02", msg)
		return
	}
	respondSuccess(ctx, "wallet upgrade request submitted successfully", map[string]interface{}{
		"user_id": userID,
		"message": resp.Message,
	})
}

// GetUserWalletUpgradeStatus GET /users/:id/wallet/upgrade-status (admin JWT) — upgrade status from 9PSB upgrade_status API (source of truth) plus optional latest request row.
func (c *AdminController) GetUserWalletUpgradeStatus(ctx *gin.Context) {
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	userID := ctx.Param("id")
	if userID == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	resp, err := c.payment.GetWalletUpgradeStatusByUserID(ctx.Request.Context(), userID)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if resp == nil {
		respondSuccess(ctx, "ok", gin.H{"has_wallet": false, "upgrade_status": nil})
		return
	}
	payload := gin.H{"has_wallet": resp.HasWallet, "upgrade_status": nil}
	if resp.UpgradeStatus != nil {
		payload["upgrade_status"] = gin.H{
			"status":  resp.UpgradeStatus.Status,
			"message": resp.UpgradeStatus.Message,
			"data": gin.H{
				"message": resp.UpgradeStatus.DataMessage,
				"status":  resp.UpgradeStatus.DataStatus,
			},
		}
	}
	if resp.Latest != nil {
		r := resp.Latest
		payload["latest_request"] = gin.H{
			"id":                r.Id,
			"wallet_id":         r.WalletId,
			"user_id":           r.UserId,
			"upgrade_ref":       r.UpgradeRef,
			"tier_from":         r.TierFrom,
			"tier_to":           r.TierTo,
			"upgrade_method":    r.UpgradeMethod,
			"initiation_status": r.InitiationStatus,
			"final_status":      r.FinalStatus,
			"initiated_by":      r.InitiatedBy,
			"submitted_at":      r.SubmittedAt,
			"finalized_at":      r.FinalizedAt,
			"created_at":        r.CreatedAt,
		}
	}
	respondSuccess(ctx, "ok", payload)
}

// ListWalletUpgradeRequests GET /wallet-upgrades (admin JWT) — list wallet upgrade requests via payment gRPC (paginated: limit, offset).
func (c *AdminController) ListWalletUpgradeRequests(ctx *gin.Context) {
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	limit, offset := int32(50), int32(0)
	if l := ctx.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = int32(n)
			if limit > 100 {
				limit = 100
			}
		}
	}
	if o := ctx.Query("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	resp, err := c.payment.ListWalletUpgradeRequests(ctx.Request.Context(), limit, offset)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if resp == nil {
		respondSuccess(ctx, "ok", gin.H{"requests": []interface{}{}, "limit": limit, "offset": offset})
		return
	}
	requests := make([]map[string]interface{}, 0, len(resp.Requests))
	for _, r := range resp.Requests {
		requests = append(requests, map[string]interface{}{
			"id":                r.Id,
			"wallet_id":         r.WalletId,
			"user_id":           r.UserId,
			"upgrade_ref":       r.UpgradeRef,
			"tier_from":         r.TierFrom,
			"tier_to":           r.TierTo,
			"upgrade_method":    r.UpgradeMethod,
			"initiation_status": r.InitiationStatus,
			"final_status":      r.FinalStatus,
			"initiated_by":      r.InitiatedBy,
			"submitted_at":      r.SubmittedAt,
			"finalized_at":      r.FinalizedAt,
			"created_at":        r.CreatedAt,
		})
	}
	respondSuccess(ctx, "ok", gin.H{"requests": requests, "limit": limit, "offset": offset})
}

// GetWalletUpgradeRequest GET /wallet-upgrades/:id (admin JWT) — get one wallet upgrade request by id (includes request/response payload JSON).
func (c *AdminController) GetWalletUpgradeRequest(ctx *gin.Context) {
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	id := ctx.Param("id")
	if id == "" {
		respondError(ctx, http.StatusBadRequest, "02", "upgrade request id required")
		return
	}
	resp, err := c.payment.GetWalletUpgradeRequest(ctx.Request.Context(), id)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if resp == nil || !resp.Found {
		respondError(ctx, http.StatusNotFound, "02", "wallet upgrade request not found")
		return
	}
	r := resp.Item
	data := map[string]interface{}{
		"id":                r.Id,
		"wallet_id":         r.WalletId,
		"user_id":           r.UserId,
		"upgrade_ref":       r.UpgradeRef,
		"tier_from":         r.TierFrom,
		"tier_to":           r.TierTo,
		"upgrade_method":    r.UpgradeMethod,
		"initiation_status": r.InitiationStatus,
		"final_status":      r.FinalStatus,
		"initiated_by":      r.InitiatedBy,
		"submitted_at":      r.SubmittedAt,
		"finalized_at":      r.FinalizedAt,
		"created_at":        r.CreatedAt,
	}
	if resp.RequestPayloadJson != "" {
		var reqPayload interface{}
		if json.Unmarshal([]byte(resp.RequestPayloadJson), &reqPayload) == nil {
			data["request_payload"] = reqPayload
		} else {
			data["request_payload"] = resp.RequestPayloadJson
		}
	}
	if resp.ResponsePayloadJson != "" {
		var respPayload interface{}
		if json.Unmarshal([]byte(resp.ResponsePayloadJson), &respPayload) == nil {
			data["response_payload"] = respPayload
		} else {
			data["response_payload"] = resp.ResponsePayloadJson
		}
	}
	respondSuccess(ctx, "ok", data)
}

// GetUserWaasTransactions GET /users/:id/wallet/waas/transactions (admin JWT) — 9PSB WaaS transaction history for user. Query: from_date, to_date (YYYY-MM-DD, max 31 days), limit (default 20).
func (c *AdminController) GetUserWaasTransactions(ctx *gin.Context) {
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	userID := ctx.Param("id")
	if userID == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	fromDate := ctx.Query("from_date")
	toDate := ctx.Query("to_date")
	if fromDate == "" || toDate == "" {
		respondError(ctx, http.StatusBadRequest, "02", "from_date and to_date (YYYY-MM-DD) are required")
		return
	}
	limit := int32(20)
	if l := ctx.Query("limit"); l != "" {
		if n, err := strconv.ParseInt(l, 10, 32); err == nil && n > 0 {
			limit = int32(n)
			if limit > 100 {
				limit = 100
			}
		}
	}
	resp, err := c.payment.GetWaasTransactionHistory(ctx.Request.Context(), userID, fromDate, toDate, limit)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Success {
		msg := resp.ErrorMessage
		if msg == "" {
			msg = "failed to fetch transaction history"
		}
		if strings.Contains(msg, "no active wallet") {
			respondError(ctx, http.StatusNotFound, "02", msg)
			return
		}
		respondError(ctx, http.StatusBadRequest, "02", msg)
		return
	}
	transactions := make([]gin.H, 0, len(resp.Transactions))
	for _, t := range resp.Transactions {
		transactions = append(transactions, gin.H{
			"transaction_date":        t.TransactionDate,
			"transaction_date_string": t.TransactionDateString,
			"amount":                  t.Amount,
			"narration":               t.Narration,
			"balance":                 t.Balance,
			"reference_id":            t.ReferenceId,
			"debit":                   t.Debit,
			"credit":                  t.Credit,
			"unique_identifier":       t.UniqueIdentifier,
			"is_reversed":             t.IsReversed,
		})
	}
	respondSuccess(ctx, resp.Message, gin.H{"transactions": transactions})
}

// GetUserWaasWalletStatus GET /users/:id/wallet/waas/status (admin JWT) — 9PSB WaaS wallet status for user.
func (c *AdminController) GetUserWaasWalletStatus(ctx *gin.Context) {
	if c.payment == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "payment service unavailable")
		return
	}
	userID := ctx.Param("id")
	if userID == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	resp, err := c.payment.GetWaasWalletStatus(ctx.Request.Context(), userID)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Success {
		msg := resp.ErrorMessage
		if msg == "" {
			msg = "failed to fetch wallet status"
		}
		if strings.Contains(msg, "no active wallet") {
			respondError(ctx, http.StatusNotFound, "02", msg)
			return
		}
		respondError(ctx, http.StatusBadRequest, "02", msg)
		return
	}
	respondSuccess(ctx, "ok", gin.H{"wallet_status": resp.WalletStatus, "response_code": resp.ResponseCode})
}

// GetUserKYC GET /users/:id/kyc (admin JWT) — full KYC for user via KYC service gRPC
func (c *AdminController) GetUserKYC(ctx *gin.Context) {
	if c.kyc == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "kyc service unavailable")
		return
	}
	id := ctx.Param("id")
	if id == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	resp, err := c.kyc.GetFullKYCForAdmin(ctx.Request.Context(), id)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Found || resp.JsonPayload == "" {
		respondSuccess(ctx, "no KYC data for this user", nil)
		return
	}
	var data interface{}
	if err := json.Unmarshal([]byte(resp.JsonPayload), &data); err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", "invalid kyc payload")
		return
	}
	respondSuccess(ctx, "ok", data)
}

// ApproveUserKYC POST /users/:id/kyc/approve (admin JWT) — sets KYC status to approved and sends success email to user. Used after wallet creation.
func (c *AdminController) ApproveUserKYC(ctx *gin.Context) {
	if c.kyc == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "kyc service unavailable")
		return
	}
	userID := ctx.Param("id")
	if userID == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id required")
		return
	}
	resp, err := c.kyc.ApproveKYC(ctx.Request.Context(), userID)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	if !resp.Success {
		msg := resp.Message
		if msg == "" {
			msg = "KYC approval failed"
		}
		if strings.Contains(msg, "not found") {
			respondError(ctx, http.StatusNotFound, "02", msg)
			return
		}
		respondError(ctx, http.StatusBadRequest, "02", msg)
		return
	}
	respondSuccess(ctx, "KYC approved successfully; user has been notified by email", map[string]interface{}{"user_id": userID, "status": "approved"})
}

// GetUserKYCImage GET /users/:id/kyc/images/:type (admin JWT) — proxies to KYC admin HTTP image endpoint using X-Admin-Key (returns binary; no ApiResponse envelope).
func (c *AdminController) GetUserKYCImage(ctx *gin.Context) {
	if c.kycKey == "" {
		respondError(ctx, http.StatusServiceUnavailable, "99", "kyc admin key not configured")
		return
	}
	userID := ctx.Param("id")
	imageType := ctx.Param("type")
	if userID == "" || imageType == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id and image type required")
		return
	}

	// Call KYC service HTTP admin endpoint directly inside the cluster.
	url := fmt.Sprintf("http://kyc-service:8002/admin/users/%s/kyc/images/%s", userID, imageType)
	req, err := http.NewRequestWithContext(ctx.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", "failed to create request")
		return
	}
	req.Header.Set("X-Admin-Key", c.kycKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		respondError(ctx, http.StatusBadGateway, "99", "failed to reach kyc service")
		return
	}
	defer resp.Body.Close()

	// Propagate KYC service status codes.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		respondError(ctx, http.StatusBadGateway, "99", "failed to read kyc response")
		return
	}

	// If KYC returned JSON (e.g. error), wrap in ApiResponse and return.
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = http.DetectContentType(body)
	}
	if resp.StatusCode >= 400 || ct == "application/json" || ct == "application/json; charset=utf-8" {
		// Forward error as common envelope if we can parse message
		var msg string
		var jsonBody struct{ Message string `json:"message"` }
		_ = json.Unmarshal(body, &jsonBody)
		if jsonBody.Message != "" {
			msg = jsonBody.Message
		} else {
			msg = string(body)
			if msg == "" {
				msg = http.StatusText(resp.StatusCode)
			}
		}
		respondError(ctx, resp.StatusCode, "99", msg)
		return
	}

	// Otherwise assume it's an image; preserve Content-Disposition when present.
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		ctx.Header("Content-Disposition", cd)
	}
	ctx.Data(resp.StatusCode, ct, body)
}

// SetStepRejectionMessage PUT /users/:id/kyc/steps/:step/rejection-message (admin JWT) — proxies to KYC admin HTTP API. Body: { "message": "..." }. Step: personal | identity | address.
func (c *AdminController) SetStepRejectionMessage(ctx *gin.Context) {
	if c.kycKey == "" {
		respondError(ctx, http.StatusServiceUnavailable, "99", "kyc admin key not configured")
		return
	}
	userID := ctx.Param("id")
	step := ctx.Param("step")
	if userID == "" || step == "" {
		respondError(ctx, http.StatusBadRequest, "02", "user id and step required")
		return
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		respondError(ctx, http.StatusBadRequest, "02", "invalid request body")
		return
	}

	url := fmt.Sprintf("http://kyc-service:8002/admin/users/%s/kyc/steps/%s/rejection-message", userID, step)
	reqBody, err := json.Marshal(map[string]string{"message": body.Message})
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", "failed to encode request")
		return
	}
	req, err := http.NewRequestWithContext(ctx.Request.Context(), http.MethodPut, url, strings.NewReader(string(reqBody)))
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", "failed to create request")
		return
	}
	req.Header.Set("X-Admin-Key", c.kycKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		respondError(ctx, http.StatusBadGateway, "99", "failed to reach kyc service")
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		respondError(ctx, http.StatusBadGateway, "99", "failed to read kyc response")
		return
	}

	if resp.StatusCode >= 400 {
		var msg string
		var jsonBody struct{ Message string `json:"message"` }
		_ = json.Unmarshal(respBody, &jsonBody)
		if jsonBody.Message != "" {
			msg = jsonBody.Message
		} else {
			msg = string(respBody)
			if msg == "" {
				msg = http.StatusText(resp.StatusCode)
			}
		}
		respondError(ctx, resp.StatusCode, "99", msg)
		return
	}

	respondSuccess(ctx, "rejection message set", nil)
}

// ListAudits GET /audits (admin JWT) — all audits or by user_id query via audit service gRPC (paginated: page, page_size)
func (c *AdminController) ListAudits(ctx *gin.Context) {
	if c.audit == nil {
		respondError(ctx, http.StatusServiceUnavailable, "99", "audit service unavailable")
		return
	}
	page, pageSize := 1, 20
	if p := ctx.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if s := ctx.Query("page_size"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			if n > 100 {
				n = 100
			}
			pageSize = n
		}
	}
	limit := int32(pageSize)
	offset := int32((page - 1) * pageSize)

	userID := ctx.Query("user_id")
	if userID != "" {
		resp, err := c.audit.GetUserAudits(ctx.Request.Context(), userID, limit, offset)
		if err != nil {
			respondError(ctx, http.StatusInternalServerError, "99", err.Error())
			return
		}
		respondSuccess(ctx, "ok", gin.H{
			"logs":     resp.Logs,
			"total":    resp.Total,
			"page":     page,
			"pageSize": pageSize,
		})
		return
	}
	resp, err := c.audit.ListAllAudits(ctx.Request.Context(), limit, offset)
	if err != nil {
		respondError(ctx, http.StatusInternalServerError, "99", err.Error())
		return
	}
	respondSuccess(ctx, "ok", gin.H{
		"logs":     resp.Logs,
		"total":    resp.Total,
		"page":     page,
		"pageSize": pageSize,
	})
}
