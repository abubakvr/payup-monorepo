package main

import (
	"context"

	kycpb "github.com/abubakvr/payup-backend/services/kyc/proto/kyc"
)

type Server struct {
	kycpb.UnimplementedKYCServiceServer
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) GetKYCStatus(
	ctx context.Context,
	req *kycpb.GetKYCStatusRequest,
) (*kycpb.GetKYCStatusResponse, error) {

	status := "approved"

	return &kycpb.GetKYCStatusResponse{
		Status: status,
	}, nil
}
