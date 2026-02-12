package repository

import "github.com/abubakvr/payup-backend/services/kyc/internal/model"

var kycs = map[string]model.KYC{}

func GetKYCByUserID(userID string) *model.KYC {
	if k, ok := kycs[userID]; ok {
		return &k
	}
	return nil
}

func SaveKYC(kyc model.KYC) error {
	kycs[kyc.UserID] = kyc
	return nil
}
