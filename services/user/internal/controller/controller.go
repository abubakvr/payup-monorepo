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

	_, err := c.svc.CreateUser(ctx.Request.Context(), req.Email, req.Password, req.Name, req.LastName, req.PhoneNumber)
	if err != nil {
		// ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}

	// ctx.JSON(http.StatusCreated, gin.H{"message": "User registered"})
	response.SuccessResponse(ctx, string(response.Success), "User registered", nil)
}

// Login handles POST /login and returns tokens via the service.
func (c *UserController) Login(ctx *gin.Context) {
	var req dto.LoginRequest
	if !validation.BindAndValidate(ctx, string(response.ValidationError), &req) {
		return
	}

	resp, err := c.svc.Login(ctx.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidCredentials) {
			// ctx.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			response.ErrorResponse(ctx, string(response.AuthenticationFailed), "invalid email or password")
			return
		}
		// ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		response.ErrorResponse(ctx, string(response.InternalServerError), err.Error())
		return
	}

	ctx.JSON(http.StatusOK, resp)
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
