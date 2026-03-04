package router

import (
	"net/http"

	"github.com/abubakvr/payup-backend/services/kyc/internal/config"
	"github.com/abubakvr/payup-backend/services/kyc/internal/controller"
	"github.com/gin-gonic/gin"
)

func SetupRouter(cfg *config.Config, ctrl *controller.KYCController) *gin.Engine {
	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "KYC Service is healthy")
	})

	// Start KYC (authenticated): validates user with user service via gRPC, creates profile. Subsequent hits use saved user_id.
	r.POST("/start", ctrl.StartKYC)

	// Flow: save/resume and steps status
	r.GET("/flow/status", ctrl.GetFlowStatus)
	r.PUT("/flow/status", ctrl.UpdateFlowStatus)
	r.GET("/steps/status", ctrl.GetStepsStatus)
	r.GET("/steps/submitted", ctrl.GetStepsSubmitted)

	// Phone verification
	r.GET("/phone", ctrl.GetPhone)
	r.POST("/phone/send-otp", ctrl.SendPhoneOTP)
	r.POST("/phone/verify-otp", ctrl.VerifyPhoneOTP)

	// BVN: verify and get (user can re-verify to update)
	r.POST("/bvn/verify", ctrl.VerifyBVN)
	r.GET("/bvn", ctrl.GetBVN)
	r.GET("/bvn/customer-image", ctrl.GetBVNCustomerImage)

	// NIN: verify and get (user can re-verify to update)
	r.POST("/nin/verify", ctrl.VerifyNIN)
	r.GET("/nin", ctrl.GetNIN)

	// Personal details
	r.GET("/personal", ctrl.GetPersonal)
	r.PUT("/personal", ctrl.UpdatePersonal)

	// Identity documents (upload one image: deletes old S3 object on re-upload, returns url + identity payload)
	r.POST("/identity/:imageType/upload", ctrl.UploadIdentityImage)
	r.GET("/identity", ctrl.GetIdentity)
	r.PUT("/identity", ctrl.UpdateIdentity)

	// Address (and address verification: utility bill + proof-of-address image upload)
	r.GET("/address", ctrl.GetAddress)
	r.PUT("/address", ctrl.UpdateAddress)
	r.GET("/address/reverse-geocode", ctrl.GetAddressGeolocation)              // get current saved geolocation
	r.POST("/address/reverse-geocode", ctrl.ReverseGeocode)                    // GPS lat/lon (+ optional accuracy) → Geoapify → store
	r.GET("/address/verification", ctrl.GetAddressVerification)
	r.POST("/address/:imageType/upload", ctrl.UploadAddressVerificationImage) // imageType: utility-bill | proof-of-address

	return r
}
