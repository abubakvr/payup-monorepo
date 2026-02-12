package router

import (
	"net/http"

	"github.com/abubakvr/payup-backend/services/user/internal/config"
	"github.com/abubakvr/payup-backend/services/user/internal/controller"
)

func SetupRouter(cfg *config.Config) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User Service is healthy"))
	})

	mux.HandleFunc("/register", controller.RegisterUser)

	return mux
}
