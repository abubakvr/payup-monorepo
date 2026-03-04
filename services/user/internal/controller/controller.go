package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/abubakvr/payup-backend/services/user/internal/auth"
	"github.com/abubakvr/payup-backend/services/user/internal/common/response"
	"github.com/abubakvr/payup-backend/services/user/internal/dto"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
	"github.com/abubakvr/payup-backend/services/user/internal/service"
	"github.com/abubakvr/payup-backend/services/user/internal/validation"

	"github.com/gin-gonic/gin"
)

// Public auth paths (no JWT required). Gateway sends X-Original-URI with e.g. /v1/users/register.
var publicAuthPaths = []string{
	"/register", "/login",
	"/password-reset", "/forgot-password", "/reset-password",
	"/verify-email", "/resend-verification", "/auth/validate",
	"/2fa/verify-login",
}

// UserController holds the user service and exposes HTTP handlers.
type UserController struct {
	svc *service.UserService
}

// NewUserController returns a new UserController with the given service.
func NewUserController(svc *service.UserService) *UserController {
	return &UserController{svc: svc}
}

// AuthValidate is called by the API gateway (nginx auth_request) to validate the request. Returns 200 to allow, 401 to deny.
// Skips JWT validation for public routes (register, login, password reset, verify email, etc.).
func (c *UserController) AuthValidate(ctx *gin.Context) {
	originalURI := ctx.GetHeader("X-Original-URI")
	if isPublicPath(originalURI) {
		ctx.Status(http.StatusOK)
		return
	}
	_, err := auth.DecodeJWTFromContext(ctx)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	ctx.Status(http.StatusOK)
}

func isPublicPath(uri string) bool {
	uri = strings.TrimSuffix(uri, "/")
	for _, p := range publicAuthPaths {
		if strings.HasSuffix(uri, p) || uri == p {
			return true
		}
	}
	return false
}

// RegisterUser handles POST /register and creates a user via the service.
func (c *UserController) RegisterUser(ctx *gin.Context) {
	var req dto.RegisterRequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}

	_, err := c.svc.CreateUser(ctx.Request.Context(), req.Email, req.Password, req.FirstName, req.LastName, req.PhoneNumber)
	if err != nil {
		// ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}

	response.SuccessResponse(ctx, string(response.Success), "Account created. Please check your email to verify your account before logging in.", nil)
}

// Login handles POST /login and returns tokens, or requires 2FA with a short-lived token.
func (c *UserController) Login(ctx *gin.Context) {
	var req dto.LoginRequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}

	result, err := c.svc.Login(ctx.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidCredentials) {
			response.AuthErrorResponse(ctx, string(response.AuthenticationFailed), "invalid email or password")
			return
		}
		if errors.Is(err, service.ErrEmailNotVerified) {
			ctx.JSON(http.StatusForbidden, gin.H{
				"status":       "error",
				"message":      "Please verify your email before logging in",
				"responseCode": string(response.AuthenticationFailed),
				"data":         nil,
			})
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	if result == nil {
		response.ErrorResponse(ctx, string(response.InternalServerError), "login failed")
		return
	}
	if result.Requires2FA != nil {
		ctx.JSON(http.StatusOK, result.Requires2FA)
		return
	}
	ctx.JSON(http.StatusOK, result.Success)
}

// VerifyEmail handles POST /verify-email with token from the verification link.
func (c *UserController) VerifyEmail(ctx *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}
	if err := c.svc.VerifyEmail(ctx.Request.Context(), req.Token); err != nil {
		switch err.Error() {
		case "invalid or expired token", "token already used", "token expired":
			response.ErrorResponse(ctx, string(response.ValidationError), err.Error())
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "Email verified. You can now log in.", nil)
}

// ResendVerification handles POST /resend-verification (resend verification email).
func (c *UserController) ResendVerification(ctx *gin.Context) {
	var req dto.ForgotPasswordRequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}
	if err := c.svc.ResendEmailVerification(ctx.Request.Context(), req.Email); err != nil {
		if err.Error() == "user not found" {
			response.ErrorResponse(ctx, string(response.ValidationError), "user not found")
			return
		}
		if err.Error() == "email already verified" {
			response.SuccessResponse(ctx, string(response.Success), "Email is already verified. You can log in.", nil)
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "Verification email sent.", nil)
}

// ForgotPassword handles POST /forgot-password (request password reset email).
func (c *UserController) ForgotPassword(ctx *gin.Context) {
	var req dto.ForgotPasswordRequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}
	if err := c.svc.SendPasswordResetEmail(ctx.Request.Context(), req.Email); err != nil {
		if err.Error() == "user not found" {
			// Don't reveal whether email exists; return success anyway
			response.SuccessResponse(ctx, string(response.Success), "If an account exists with this email, you will receive a reset link.", nil)
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "If an account exists with this email, you will receive a reset link.", nil)
}

// ResetPassword handles POST /reset-password (set new password with token).
func (c *UserController) ResetPassword(ctx *gin.Context) {
	var req dto.ResetPasswordRequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}
	if err := c.svc.ResetPassword(ctx.Request.Context(), req.Token, req.NewPassword); err != nil {
		switch err.Error() {
		case "invalid or expired token", "token already used", "token expired":
			response.ErrorResponse(ctx, string(response.ValidationError), err.Error())
			return
		case "user not found":
			response.ErrorResponse(ctx, string(response.ValidationError), "invalid or expired token")
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "Password has been reset.", nil)
}

// ChangePassword handles POST /change-password (authenticated). Requires JWT; user sends old_password and new_password.
func (c *UserController) ChangePassword(ctx *gin.Context) {
	claims, err := auth.DecodeJWTFromContext(ctx)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var req dto.ChangePasswordRequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}
	if err := c.svc.ChangePassword(ctx.Request.Context(), claims.Email, req.OldPassword, req.NewPassword); err != nil {
		if err.Error() == "invalid current password" {
			response.ErrorResponse(ctx, string(response.AuthenticationFailed), "Current password is incorrect")
			return
		}
		if err.Error() == "user not found" {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "Password has been changed.", nil)
}

// GetSettings returns the authenticated user's settings (GET /settings). Requires JWT.
func (c *UserController) GetSettings(ctx *gin.Context) {
	claims, err := auth.DecodeJWTFromContext(ctx)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	settings, err := c.svc.GetSettings(ctx.Request.Context(), claims.UserID)
	if err != nil {
		if err.Error() == "user settings not found" {
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "Settings retrieved.", settings)
}

// UpdateSettings applies a partial update to the authenticated user's settings (PATCH /settings). Requires JWT.
func (c *UserController) UpdateSettings(ctx *gin.Context) {
	claims, err := auth.DecodeJWTFromContext(ctx)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var req dto.UpdateSettingsRequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}
	settings, err := c.svc.UpdateSettings(ctx.Request.Context(), claims.UserID, &req)
	if err != nil {
		if err.Error() == "user settings not found" {
			ctx.AbortWithStatus(http.StatusNotFound)
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "Settings updated.", settings)
}

// Setup2FA handles POST /2fa/setup (authenticated). Returns TOTP secret and QR URL for authenticator app.
func (c *UserController) Setup2FA(ctx *gin.Context) {
	claims, err := auth.DecodeJWTFromContext(ctx)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	secret, qrCodeURL, err := c.svc.Setup2FA(ctx.Request.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, service.Err2FAAlreadyEnabled) {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status": "error", "message": err.Error(), "responseCode": string(response.ValidationError),
			})
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "Scan the QR code or enter the secret in your app.", dto.Setup2FAResponse{
		Secret:    secret,
		QRCodeURL: qrCodeURL,
		Message:   "Enter the 6-digit code from your app in POST /2fa/verify-setup to enable 2FA.",
	})
}

// VerifySetup2FA handles POST /2fa/verify-setup (authenticated). Verifies TOTP code and enables 2FA.
func (c *UserController) VerifySetup2FA(ctx *gin.Context) {
	claims, err := auth.DecodeJWTFromContext(ctx)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var req dto.VerifySetup2FARequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}
	if err := c.svc.VerifySetup2FA(ctx.Request.Context(), claims.UserID, req.Code); err != nil {
		if errors.Is(err, service.ErrInvalidTOTPCode) {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status": "error", "message": "Invalid or expired code", "responseCode": string(response.ValidationError),
			})
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "Two-factor authentication is now enabled.", nil)
}

// VerifyLogin2FA handles POST /2fa/verify-login (no JWT; uses twoFactorToken from login response). Returns access and refresh tokens.
func (c *UserController) VerifyLogin2FA(ctx *gin.Context) {
	var req dto.VerifyLogin2FARequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}
	resp, err := c.svc.VerifyLogin2FA(ctx.Request.Context(), req.TwoFactorToken, req.Code)
	if err != nil {
		if errors.Is(err, service.ErrInvalidTOTPCode) {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status": "error", "message": "Invalid or expired code", "responseCode": string(response.ValidationError),
			})
			return
		}
		ctx.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// Disable2FA handles POST /2fa/disable (authenticated). Requires password to turn off 2FA.
func (c *UserController) Disable2FA(ctx *gin.Context) {
	claims, err := auth.DecodeJWTFromContext(ctx)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var req dto.Disable2FARequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}
	if err := c.svc.Disable2FA(ctx.Request.Context(), claims.UserID, req.Password); err != nil {
		if errors.Is(err, service.Err2FANotEnabled) {
			ctx.JSON(http.StatusBadRequest, gin.H{
				"status": "error", "message": err.Error(), "responseCode": string(response.ValidationError),
			})
			return
		}
		if err.Error() == "invalid password" {
			response.ErrorResponse(ctx, string(response.AuthenticationFailed), "Invalid password")
			return
		}
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}
	response.SuccessResponse(ctx, string(response.Success), "Two-factor authentication has been disabled.", nil)
}
