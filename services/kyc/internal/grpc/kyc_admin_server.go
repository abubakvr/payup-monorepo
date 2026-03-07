package grpc

import (
	"context"
	"encoding/json"

	kycpb "github.com/abubakvr/payup-backend/proto/kyc"
	"github.com/abubakvr/payup-backend/services/kyc/internal/service"
)

// KYCAdminServer implements kycpb.KYCServiceServer for Admin service (GetFullKYCForAdmin). GetKYCStatus returns a stub.
type KYCAdminServer struct {
	kycpb.UnimplementedKYCServiceServer
	svc *service.KYCService
}

func NewKYCAdminServer(svc *service.KYCService) *KYCAdminServer {
	return &KYCAdminServer{svc: svc}
}

func (s *KYCAdminServer) GetKYCStatus(ctx context.Context, req *kycpb.GetKYCStatusRequest) (*kycpb.GetKYCStatusResponse, error) {
	// Status can be derived from flow if needed; stub for admin gRPC.
	return &kycpb.GetKYCStatusResponse{Status: "pending"}, nil
}

func (s *KYCAdminServer) GetFullKYCForAdmin(ctx context.Context, req *kycpb.GetFullKYCForAdminRequest) (*kycpb.GetFullKYCForAdminResponse, error) {
	if req == nil || req.UserId == "" {
		return &kycpb.GetFullKYCForAdminResponse{Found: false}, nil
	}
	data, err := s.svc.GetFullKYCByUserID(req.UserId)
	if err != nil || data == nil {
		return &kycpb.GetFullKYCForAdminResponse{Found: false}, nil
	}
	jsonPayload, err := json.Marshal(data)
	if err != nil {
		return &kycpb.GetFullKYCForAdminResponse{Found: false}, nil
	}
	return &kycpb.GetFullKYCForAdminResponse{
		Found:       true,
		JsonPayload: string(jsonPayload),
	}, nil
}

func (s *KYCAdminServer) CountProfiles(ctx context.Context, req *kycpb.CountProfilesRequest) (*kycpb.CountProfilesResponse, error) {
	if req == nil {
		return &kycpb.CountProfilesResponse{Count: 0}, nil
	}
	status := req.GetStatus()
	var kycLevel *int32
	if req.KycLevel != nil {
		kycLevel = req.KycLevel
	}
	count, err := s.svc.CountProfiles(status, kycLevel)
	if err != nil {
		return nil, err
	}
	return &kycpb.CountProfilesResponse{Count: count}, nil
}

func (s *KYCAdminServer) GetKYCForWallet(ctx context.Context, req *kycpb.GetKYCForWalletRequest) (*kycpb.GetKYCForWalletResponse, error) {
	if req == nil || req.UserId == "" {
		return &kycpb.GetKYCForWalletResponse{Found: false}, nil
	}
	data, err := s.svc.GetKYCForWallet(req.UserId)
	if err != nil || data == nil {
		return &kycpb.GetKYCForWalletResponse{Found: false}, nil
	}
	return &kycpb.GetKYCForWalletResponse{
		Found:              true,
		Bvn:                data.BVN,
		DateOfBirth:        data.DateOfBirth,
		Gender:             data.Gender,
		LastName:           data.LastName,
		OtherNames:         data.OtherNames,
		PhoneNo:            data.PhoneNo,
		PlaceOfBirth:       data.PlaceOfBirth,
		Address:            data.Address,
		NationalIdentityNo: data.NationalIdentityNo,
		NinUserId:          data.NinUserID,
		NextOfKinPhoneNo:   data.NextOfKinPhoneNo,
		NextOfKinName:      data.NextOfKinName,
		Email:              data.Email,
	}, nil
}

func (s *KYCAdminServer) GetKYCForWalletUpgrade(ctx context.Context, req *kycpb.GetKYCForWalletUpgradeRequest) (*kycpb.GetKYCForWalletUpgradeResponse, error) {
	if req == nil || req.UserId == "" {
		return &kycpb.GetKYCForWalletUpgradeResponse{Found: false}, nil
	}
	data, err := s.svc.GetKYCForWalletUpgrade(ctx, req.UserId, "")
	if err != nil || data == nil {
		return &kycpb.GetKYCForWalletUpgradeResponse{Found: false}, nil
	}
	return &kycpb.GetKYCForWalletUpgradeResponse{
		Found:             true,
		AccountName:       data.AccountName,
		Bvn:               data.BVN,
		City:              data.City,
		Email:             data.Email,
		HouseNumber:       data.HouseNumber,
		IdIssueDate:       data.IDIssueDate,
		IdNumber:          data.IDNumber,
		IdType:            data.IDType,
		LocalGovernment:   data.LocalGovernment,
		Pep:               data.PEP,
		PhoneNumber:       data.PhoneNumber,
		State:             data.State,
		StreetName:        data.StreetName,
		Tier:              data.Tier,
		IdExpiryDate:      data.IDExpiryDate,
		NearestLandmark:   data.NearestLandmark,
		PlaceOfBirth:      data.PlaceOfBirth,
		Nin:               data.NIN,
		IdFrontImage:       data.IDFrontImage,
		IdBackImage:        data.IDBackImage,
		CustomerImage:      data.CustomerImage,
		UtilityBillImage:     data.UtilityBillImage,
		ProofOfAddressImage:  data.ProofOfAddressImage,
	}, nil
}

func (s *KYCAdminServer) ApproveKYC(ctx context.Context, req *kycpb.ApproveKYCRequest) (*kycpb.ApproveKYCResponse, error) {
	if req == nil || req.UserId == "" {
		return &kycpb.ApproveKYCResponse{Success: false, Message: "user_id required"}, nil
	}
	success, errMsg := s.svc.ApproveKYC(ctx, req.UserId)
	if !success {
		return &kycpb.ApproveKYCResponse{Success: false, Message: errMsg}, nil
	}
	return &kycpb.ApproveKYCResponse{Success: true}, nil
}
