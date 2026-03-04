package grpc

import (
	"context"

	userpb "github.com/abubakvr/payup-backend/proto/user"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
)

// KYCUserServer implements user.UserServiceForKYC for KYC service to validate and fetch user by ID.
type KYCUserServer struct {
	userpb.UnimplementedUserServiceForKYCServer
	userRepo *repository.UserRepository
}

// Ensure KYCUserServer implements the interface.
var _ userpb.UserServiceForKYCServer = (*KYCUserServer)(nil)

func NewKYCUserServer(userRepo *repository.UserRepository) *KYCUserServer {
	return &KYCUserServer{userRepo: userRepo}
}

func (s *KYCUserServer) GetUserForKYC(ctx context.Context, req *userpb.GetUserForKYCRequest) (*userpb.GetUserForKYCResponse, error) {
	if req == nil || req.UserId == "" {
		return &userpb.GetUserForKYCResponse{Found: false}, nil
	}
	user, err := s.userRepo.GetUserByID(req.UserId)
	if err != nil || user == nil {
		return &userpb.GetUserForKYCResponse{Found: false}, nil
	}
	return &userpb.GetUserForKYCResponse{
		Found:     true,
		UserId:    user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}, nil
}
