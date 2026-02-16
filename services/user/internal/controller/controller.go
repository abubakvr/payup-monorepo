package controller

import (
	"encoding/json"
	"net/http"

	"github.com/abubakvr/payup-backend/services/user/internal/dto"
	"github.com/abubakvr/payup-backend/services/user/internal/service"
)

// AuthValidate is called by the API gateway (nginx auth_request) to validate the request. Returns 200 to allow, 401 to deny.
func AuthValidate(w http.ResponseWriter, r *http.Request) {
	// Stub: allow all. Replace with JWT/session validation later.
	w.WriteHeader(http.StatusOK)
}

func RegisterUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req dto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request"))
		return
	}

	err := service.CreateUser(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("User registered"))
}
