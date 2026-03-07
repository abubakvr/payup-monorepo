package grpc

import (
	"context"

	paymentpb "github.com/abubakvr/payup-backend/proto/payment"
	"github.com/abubakvr/payup-backend/services/payment/internal/kafka"
	"github.com/abubakvr/payup-backend/services/payment/internal/service"
)

// Server implements paymentpb.PaymentServiceServer. Add your RPC handlers here.
type Server struct {
	paymentpb.UnimplementedPaymentServiceServer
	svc *service.PaymentService
}

// NewServer returns a new gRPC server for the payment service.
func NewServer(svc *service.PaymentService) *Server {
	return &Server{svc: svc}
}

// Health returns service health status.
func (s *Server) Health(ctx context.Context, req *paymentpb.HealthRequest) (*paymentpb.HealthResponse, error) {
	if err := s.svc.Health(ctx); err != nil {
		return &paymentpb.HealthResponse{Status: "unhealthy"}, nil
	}
	return &paymentpb.HealthResponse{Status: "ok"}, nil
}

// CreateWallet creates a 9PSB wallet for the user (fetches KYC via gRPC, calls 9PSB, saves wallet; audit + email via Kafka).
func (s *Server) CreateWallet(ctx context.Context, req *paymentpb.CreateWalletRequest) (*paymentpb.CreateWalletResponse, error) {
	if req == nil || req.UserId == "" {
		return &paymentpb.CreateWalletResponse{Success: false, ErrorMessage: "user_id required"}, nil
	}
	accountNumber, err := s.svc.CreateWallet(ctx, req.UserId)
	if err != nil {
		_ = s.svc.SendAuditLog(kafka.AuditLogParams{
			Service:  "payment",
			Action:   "wallet_creation_failed",
			Entity:   "wallet",
			EntityID: "",
			UserID:   &req.UserId,
			Metadata: map[string]interface{}{"error": err.Error()},
		})
		return &paymentpb.CreateWalletResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &paymentpb.CreateWalletResponse{Success: true, AccountNumber: accountNumber}, nil
}

// ListWallets returns all wallets with details for admin view (paginated).
func (s *Server) ListWallets(ctx context.Context, req *paymentpb.ListWalletsRequest) (*paymentpb.ListWalletsResponse, error) {
	limit := int(req.GetLimit())
	offset := int(req.GetOffset())
	list, err := s.svc.ListWalletsForAdmin(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*paymentpb.WalletDetail, 0, len(list))
	for _, w := range list {
		out = append(out, &paymentpb.WalletDetail{
			Id:                w.ID,
			UserId:            w.UserID,
			AccountNumber:     w.AccountNumber,
			CustomerId:        w.CustomerID,
			OrderRef:          w.OrderRef,
			FullName:          w.FullName,
			Phone:             w.Phone,
			Email:             w.Email,
			MfbCode:           w.MfbCode,
			Tier:              w.Tier,
			Status:            w.Status,
			LedgerBalance:     w.LedgerBalance,
			AvailableBalance:  w.AvailableBalance,
			Provider:          w.Provider,
			CreatedAt:         w.CreatedAt,
			UpdatedAt:         w.UpdatedAt,
		})
	}
	return &paymentpb.ListWalletsResponse{Wallets: out}, nil
}
