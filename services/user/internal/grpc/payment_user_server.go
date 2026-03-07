package grpc

import (
	"context"

	userpb "github.com/abubakvr/payup-backend/proto/user"
	"github.com/abubakvr/payup-backend/services/user/internal/service"
)

// PaymentUserServer implements user.UserServiceForPayment for Payment service (validate transfer: PIN, restricted, paused).
type PaymentUserServer struct {
	userpb.UnimplementedUserServiceForPaymentServer
	userSvc *service.UserService
}

// NewPaymentUserServer returns a new PaymentUserServer.
func NewPaymentUserServer(userSvc *service.UserService) *PaymentUserServer {
	return &PaymentUserServer{userSvc: userSvc}
}

// ValidateTransfer checks user exists, not banking restricted, transfers not paused, and PIN matches.
// Returns daily/monthly limits so payment service can enforce them against its transaction data.
func (s *PaymentUserServer) ValidateTransfer(ctx context.Context, req *userpb.ValidateTransferRequest) (*userpb.ValidateTransferResponse, error) {
	if req == nil || req.UserId == "" {
		return &userpb.ValidateTransferResponse{Allowed: false, Message: "user_id required"}, nil
	}
	allowed, message, dailyLimit, monthlyLimit := s.userSvc.ValidateTransfer(ctx, req.UserId, req.Amount, req.Pin)
	return &userpb.ValidateTransferResponse{
		Allowed:             allowed,
		Message:             message,
		DailyTransferLimit:  dailyLimit,
		MonthlyTransferLimit: monthlyLimit,
	}, nil
}
