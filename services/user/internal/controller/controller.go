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
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		response.ErrorResponse(ctx, string(response.ValidationError), "Invalid request")
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
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		response.ErrorResponse(ctx, string(response.ValidationError), "Invalid request")
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
