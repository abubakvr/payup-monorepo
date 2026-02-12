package controller

import (
	"encoding/json"
	"net/http"

	"github.com/abubakvr/payup-backend/services/kyc/internal/dto"
	"github.com/abubakvr/payup-backend/services/kyc/internal/service"
)

func SubmitKYC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req dto.KYCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Invalid request"))
		return
	}

	err := service.SubmitKYC(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("KYC submitted"))
}
