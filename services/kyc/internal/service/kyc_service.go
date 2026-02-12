package service

import (
	"errors"

	"github.com/abubakvr/payup-backend/services/kyc/internal/dto"
	"github.com/abubakvr/payup-backend/services/kyc/internal/model"
	"github.com/abubakvr/payup-backend/services/kyc/internal/repository"
)

func SubmitKYC(req dto.KYCRequest) error {
	exists := repository.GetKYCByUserID(req.UserID)
	if exists != nil {
		return errors.New("KYC already submitted for this user")
	}

	kyc := model.KYC{
		UserID:   req.UserID,
		Document: req.Document,
		DocType:  req.Doctype,
		FullName: req.FullName,
		Status:   "pending",
	}

	return repository.SaveKYC(kyc)
}
