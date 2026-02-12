package router

import (
	"net/http"

	"github.com/abubakvr/payup-backend/services/kyc/internal/config"
	"github.com/abubakvr/payup-backend/services/kyc/internal/controller"
)

func SetupRouter(cfg *config.Config) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("KYC Service is healthy"))
	})

	mux.HandleFunc("/kyc/submit", controller.SubmitKYC)

	return mux
}
