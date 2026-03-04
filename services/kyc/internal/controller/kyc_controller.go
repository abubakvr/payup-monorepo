package controller

import (
	"io"
	"net/http"

	"github.com/abubakvr/payup-backend/services/kyc/internal/auth"
	"github.com/abubakvr/payup-backend/services/kyc/internal/common/response"
	"github.com/abubakvr/payup-backend/services/kyc/internal/dto"
	"github.com/abubakvr/payup-backend/services/kyc/internal/repository"
	"github.com/abubakvr/payup-backend/services/kyc/internal/service"
	"github.com/abubakvr/payup-backend/services/kyc/internal/validation"
	"github.com/gin-gonic/gin"
)

type KYCController struct {
	svc *service.KYCService
}

func NewKYCController(svc *service.KYCService) *KYCController {
	return &KYCController{svc: svc}
}

func (c *KYCController) withUserID(ctx *gin.Context) (userID string, ok bool) {
	claims, err := auth.DecodeJWTFromContext(ctx.GetHeader)
	if err != nil {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return "", false
	}
	return claims.UserID, true
}

func (c *KYCController) handleKYCError(ctx *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if err == service.ErrKYCNotStarted {
		ctx.JSON(http.StatusNotFound, gin.H{
			"status": "error", "message": err.Error(), "responseCode": response.ValidationError,
		})
		return true
	}
	if err == service.ErrUserNotFound {
		ctx.JSON(http.StatusNotFound, gin.H{
			"status": "error", "message": "User not found", "responseCode": response.ValidationError,
		})
		return true
	}
	response.ErrorResponse(ctx, response.ValidationError, err.Error())
	return true
}

// StartKYC POST /start (authenticated). Validates user with user service via gRPC and creates KYC profile. Subsequent hits use saved user_id.
func (c *KYCController) StartKYC(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.StartKYC(ctx.Request.Context(), userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{
		"status": "success", "message": "KYC started", "responseCode": response.Success, "data": data,
	})
}

// GetFlowStatus GET /flow/status
func (c *KYCController) GetFlowStatus(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetFlowStatus(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// UpdateFlowStatus PUT /flow/status (save/resume)
func (c *KYCController) UpdateFlowStatus(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	var req dto.UpdateFlowStatusRequest
	if !validation.BindAndValidate(ctx, response.ValidationError, &req) {
		return
	}
	data, err := c.svc.UpdateFlowStatus(userID, &req)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "Flow status updated", data)
}

// GetStepsStatus GET /steps/status
func (c *KYCController) GetStepsStatus(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetStepsStatus(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// GetStepsSubmitted GET /steps/submitted — returns list of steps with submitted and verified flags (from KYC tables).
func (c *KYCController) GetStepsSubmitted(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetStepsSubmitted(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// VerifyBVN POST /bvn/verify (allows re-verify to update)
func (c *KYCController) VerifyBVN(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	var req dto.BVNVerifyRequest
	if !validation.BindAndValidate(ctx, response.ValidationError, &req) {
		return
	}
	data, err := c.svc.VerifyBVN(userID, &req)
	if err != nil {
		if err == repository.ErrEncryptionKeyMissing {
			response.ErrorResponse(ctx, response.ValidationError, "Server configuration error")
			return
		}
		if c.handleKYCError(ctx, err) {
			return
		}
		response.ErrorResponse(ctx, response.ValidationError, err.Error())
		return
	}
	response.SuccessResponse(ctx, "BVN verified successfully", data)
}

// GetBVN GET /bvn
func (c *KYCController) GetBVN(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetBVN(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// GetBVNCustomerImage GET /bvn/customer-image — returns the decrypted selfie image (when stored encrypted in S3).
func (c *KYCController) GetBVNCustomerImage(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	body, contentType, err := c.svc.GetBVNCustomerImage(ctx.Request.Context(), userID)
	if err != nil {
		if c.handleKYCError(ctx, err) {
			return
		}
		response.ErrorResponse(ctx, response.ValidationError, err.Error())
		return
	}
	if body == nil {
		ctx.Status(http.StatusNotFound)
		return
	}
	if contentType == "" {
		contentType = "image/jpeg"
	}
	ctx.Data(http.StatusOK, contentType, body)
}

// VerifyNIN POST /nin/verify (allows re-verify to update)
func (c *KYCController) VerifyNIN(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	var req dto.NINVerifyRequest
	if !validation.BindAndValidate(ctx, response.ValidationError, &req) {
		return
	}
	if err := c.svc.VerifyNIN(userID, &req); err != nil {
		if err == repository.ErrEncryptionKeyMissing {
			response.ErrorResponse(ctx, response.ValidationError, "Server configuration error")
			return
		}
		if c.handleKYCError(ctx, err) {
			return
		}
		response.ErrorResponse(ctx, response.ValidationError, err.Error())
		return
	}
	response.SuccessResponse(ctx, "NIN verified successfully", nil)
}

// GetNIN GET /nin
func (c *KYCController) GetNIN(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetNIN(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// SendPhoneOTP POST /phone/send-otp
func (c *KYCController) SendPhoneOTP(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	var req dto.PhoneSendOTPRequest
	if !validation.BindAndValidate(ctx, response.ValidationError, &req) {
		return
	}
	if err := c.svc.SendPhoneOTP(userID, &req); c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OTP sent", nil)
}

// VerifyPhoneOTP POST /phone/verify-otp
func (c *KYCController) VerifyPhoneOTP(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	var req dto.PhoneVerifyOTPRequest
	if !validation.BindAndValidate(ctx, response.ValidationError, &req) {
		return
	}
	if err := c.svc.VerifyPhoneOTP(userID, &req); c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "Phone verified", nil)
}

// GetPhone GET /phone — returns verification status, masked phone, and submitted flag.
func (c *KYCController) GetPhone(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetPhone(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// GetPersonal GET /personal
func (c *KYCController) GetPersonal(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetPersonal(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// UpdatePersonal PUT /personal
func (c *KYCController) UpdatePersonal(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	var req dto.PersonalDetailsRequest
	if !validation.BindAndValidate(ctx, response.ValidationError, &req) {
		return
	}
	if err := c.svc.UpdatePersonal(userID, &req); c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "Personal details updated", nil)
}

// GetIdentity GET /identity
func (c *KYCController) GetIdentity(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetIdentity(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// UpdateIdentity PUT /identity
func (c *KYCController) UpdateIdentity(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	var req dto.IdentityDocumentsRequest
	if !validation.BindAndValidate(ctx, response.ValidationError, &req) {
		return
	}
	if err := c.svc.UpdateIdentity(userID, &req); c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "Identity documents updated", nil)
}

// UploadIdentityImage POST /identity/:imageType/upload — multipart form with one file (field "file"). Returns { url, data } for frontend; re-upload deletes old S3 object.
func (c *KYCController) UploadIdentityImage(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	imageTypeParam := ctx.Param("imageType")
	var imageType string
	switch imageTypeParam {
	case "id-front":
		imageType = service.IdentityImageTypeFront
	case "id-back":
		imageType = service.IdentityImageTypeBack
	case "customer-image":
		imageType = service.IdentityImageTypeCustomer
	case "signature":
		imageType = service.IdentityImageTypeSignature
	default:
		ctx.Status(http.StatusNotFound)
		return
	}
	const maxFormMemory = 10 << 20
	if err := ctx.Request.ParseMultipartForm(maxFormMemory); err != nil {
		response.ErrorResponse(ctx, response.ValidationError, "invalid form: "+err.Error())
		return
	}
	idType := ctx.Request.FormValue("idType")
	if idType != "" && idType != "passport" && idType != "drivers_license" && idType != "national_id" {
		response.ErrorResponse(ctx, response.ValidationError, "idType must be passport, drivers_license, or national_id")
		return
	}
	fh, err := ctx.FormFile("file")
	if err != nil || fh == nil {
		response.ErrorResponse(ctx, response.ValidationError, "file is required")
		return
	}
	file, err := fh.Open()
	if err != nil {
		response.ErrorResponse(ctx, response.ValidationError, "failed to read file")
		return
	}
	body, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil {
		response.ErrorResponse(ctx, response.ValidationError, "failed to read file")
		return
	}
	if len(body) == 0 {
		response.ErrorResponse(ctx, response.ValidationError, "file is empty")
		return
	}
	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	uploadedURL, err := c.svc.UploadIdentityImageSlot(ctx.Request.Context(), userID, imageType, idType, body, contentType)
	if c.handleKYCError(ctx, err) {
		return
	}
	identityData, _ := c.svc.GetIdentity(userID)
	resp := dto.IdentityImageUploadResponse{URL: uploadedURL}
	if identityData != nil {
		resp.Data = *identityData
	}
	response.SuccessResponse(ctx, "Image uploaded", resp)
}

// GetAddress GET /address
func (c *KYCController) GetAddress(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetAddress(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// UpdateAddress PUT /address — returns saved address including utilityBillUrl and proofOfAddressUrl when provided.
func (c *KYCController) UpdateAddress(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	var req dto.AddressRequest
	if !validation.BindAndValidate(ctx, response.ValidationError, &req) {
		return
	}
	data, err := c.svc.UpdateAddress(userID, &req)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "Address updated", data)
}

// GetAddressGeolocation GET /address/reverse-geocode — returns current saved reverse-geocoded address (or empty when none).
func (c *KYCController) GetAddressGeolocation(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetAddressGeolocation(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	if data == nil {
		response.SuccessResponse(ctx, "OK", nil)
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// ReverseGeocode POST /address/reverse-geocode — accepts lat, lon, optional accuracy from frontend; calls Geoapify, stores in kyc_address_geolocations.
func (c *KYCController) ReverseGeocode(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	var req dto.ReverseGeocodeRequest
	if !validation.BindAndValidate(ctx, response.ValidationError, &req) {
		return
	}
	ip := ctx.ClientIP()
	userAgent := ctx.GetHeader("User-Agent")
	data, err := c.svc.ReverseGeocode(userID, &req, ip, userAgent)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// GetAddressVerification GET /address/verification — returns utility bill URL, proof-of-address URL, verification status.
func (c *KYCController) GetAddressVerification(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	data, err := c.svc.GetAddressVerification(userID)
	if c.handleKYCError(ctx, err) {
		return
	}
	response.SuccessResponse(ctx, "OK", data)
}

// UploadAddressVerificationImage POST /address/utility-bill/upload or /address/proof-of-address/upload — multipart form "file". Returns { url, data }.
func (c *KYCController) UploadAddressVerificationImage(ctx *gin.Context) {
	userID, ok := c.withUserID(ctx)
	if !ok {
		return
	}
	imageTypeParam := ctx.Param("imageType")
	var imageType string
	switch imageTypeParam {
	case "utility-bill":
		imageType = service.AddressVerificationImageUtilityBill
	case "proof-of-address":
		imageType = service.AddressVerificationImageProofOfAddress
	default:
		ctx.Status(http.StatusNotFound)
		return
	}
	const maxFormMemory = 10 << 20
	if err := ctx.Request.ParseMultipartForm(maxFormMemory); err != nil {
		response.ErrorResponse(ctx, response.ValidationError, "invalid form: "+err.Error())
		return
	}
	fh, err := ctx.FormFile("file")
	if err != nil || fh == nil {
		response.ErrorResponse(ctx, response.ValidationError, "file is required")
		return
	}
	file, err := fh.Open()
	if err != nil {
		response.ErrorResponse(ctx, response.ValidationError, "failed to read file")
		return
	}
	body, err := io.ReadAll(file)
	_ = file.Close()
	if err != nil {
		response.ErrorResponse(ctx, response.ValidationError, "failed to read file")
		return
	}
	if len(body) == 0 {
		response.ErrorResponse(ctx, response.ValidationError, "file is empty")
		return
	}
	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	uploadedURL, err := c.svc.UploadAddressVerificationImageSlot(ctx.Request.Context(), userID, imageType, body, contentType)
	if c.handleKYCError(ctx, err) {
		return
	}
	verifData, _ := c.svc.GetAddressVerification(userID)
	resp := dto.AddressVerificationUploadResponse{URL: uploadedURL}
	if verifData != nil {
		resp.Data = *verifData
	}
	response.SuccessResponse(ctx, "Image uploaded", resp)
}
