package grpc

import (
	"context"

	userpb "github.com/abubakvr/payup-backend/proto/user"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
	"github.com/abubakvr/payup-backend/services/user/internal/service"
)

// AdminUserServer implements user.UserServiceForAdmin for Admin service (list users, get user, set restricted).
type AdminUserServer struct {
	userpb.UnimplementedUserServiceForAdminServer
	userRepo *repository.UserRepository
	userSvc  *service.UserService
}

func NewAdminUserServer(userRepo *repository.UserRepository, userSvc *service.UserService) *AdminUserServer {
	return &AdminUserServer{userRepo: userRepo, userSvc: userSvc}
}

func (s *AdminUserServer) ListUsers(ctx context.Context, req *userpb.ListUsersRequest) (*userpb.ListUsersResponse, error) {
	limit, offset := 50, 0
	if req != nil {
		if req.Limit > 0 {
			limit = int(req.Limit)
		}
		if req.Offset >= 0 {
			offset = int(req.Offset)
		}
	}
	total, err := s.userRepo.CountUsers()
	if err != nil {
		return nil, err
	}
	users, err := s.userRepo.ListUsers(limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*userpb.AdminUserSummary, len(users))
	for i := range users {
		out[i] = &userpb.AdminUserSummary{
			Id:                users[i].ID,
			Email:             users[i].Email,
			FirstName:         users[i].FirstName,
			LastName:          users[i].LastName,
			PhoneNumber:       users[i].PhoneNumber,
			EmailVerified:     users[i].EmailVerified,
			CreatedAt:         users[i].CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:         users[i].UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			BankingRestricted: users[i].BankingRestricted,
		}
	}
	return &userpb.ListUsersResponse{Users: out, Total: int32(total)}, nil
}

func (s *AdminUserServer) GetUserForAdmin(ctx context.Context, req *userpb.GetUserForAdminRequest) (*userpb.GetUserForAdminResponse, error) {
	if req == nil || req.UserId == "" {
		return &userpb.GetUserForAdminResponse{Found: false}, nil
	}
	user, err := s.userRepo.GetUserByID(req.UserId)
	if err != nil || user == nil {
		return &userpb.GetUserForAdminResponse{Found: false}, nil
	}
	return &userpb.GetUserForAdminResponse{
		Found: true,
		User: &userpb.AdminUserSummary{
			Id:                user.ID,
			Email:             user.Email,
			FirstName:         user.FirstName,
			LastName:          user.LastName,
			PhoneNumber:       user.PhoneNumber,
			EmailVerified:     user.EmailVerified,
			CreatedAt:         user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:         user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			BankingRestricted: user.BankingRestricted,
		},
	}, nil
}

func (s *AdminUserServer) SetUserRestricted(ctx context.Context, req *userpb.SetUserRestrictedRequest) (*userpb.SetUserRestrictedResponse, error) {
	if req == nil || req.UserId == "" {
		return &userpb.SetUserRestrictedResponse{Success: false, Message: "user_id required"}, nil
	}
	if err := s.userSvc.SetUserRestricted(ctx, req.UserId, req.Restricted); err != nil {
		if err == repository.ErrUserNotFound {
			return &userpb.SetUserRestrictedResponse{Success: false, Message: "user not found"}, nil
		}
		return &userpb.SetUserRestrictedResponse{Success: false, Message: err.Error()}, nil
	}
	return &userpb.SetUserRestrictedResponse{Success: true}, nil
}
