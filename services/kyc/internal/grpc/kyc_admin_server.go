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
