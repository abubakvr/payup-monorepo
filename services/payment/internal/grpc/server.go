package grpc

import (
	"context"
	"strings"
	"time"

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

// DebitCreditWallet performs an internal debit or credit on the user's wallet (airtime, data, electricity, DSTV, admin adjust). Saves to transactions and ledger.
func (s *Server) DebitCreditWallet(ctx context.Context, req *paymentpb.DebitCreditWalletRequest) (*paymentpb.DebitCreditWalletResponse, error) {
	if req == nil || req.UserId == "" {
		return &paymentpb.DebitCreditWalletResponse{Success: false, ErrorMessage: "user_id required"}, nil
	}
	if req.Amount <= 0 {
		return &paymentpb.DebitCreditWalletResponse{Success: false, ErrorMessage: "amount must be positive"}, nil
	}
	if req.Narration == "" {
		return &paymentpb.DebitCreditWalletResponse{Success: false, ErrorMessage: "narration is required"}, nil
	}
	result, err := s.svc.WalletDebitCredit(ctx, req.UserId, req.Amount, req.IsCredit, req.Narration, req.InitiatedBy)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "insufficient balance") {
			return &paymentpb.DebitCreditWalletResponse{Success: false, ErrorMessage: msg}, nil
		}
		if strings.Contains(msg, "no active wallet") {
			return &paymentpb.DebitCreditWalletResponse{Success: false, ErrorMessage: msg}, nil
		}
		return &paymentpb.DebitCreditWalletResponse{Success: false, ErrorMessage: msg}, nil
	}
	return &paymentpb.DebitCreditWalletResponse{Success: true, TransactionRef: result.TransactionRef}, nil
}

// GetWaasTransactionHistory returns 9PSB WaaS transaction history for the given user's wallet (admin). Date range max 31 days.
func (s *Server) GetWaasTransactionHistory(ctx context.Context, req *paymentpb.GetWaasTransactionHistoryRequest) (*paymentpb.GetWaasTransactionHistoryResponse, error) {
	if req == nil || req.UserId == "" {
		return &paymentpb.GetWaasTransactionHistoryResponse{Success: false, ErrorMessage: "user_id required"}, nil
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	result, err := s.svc.GetWaasTransactionHistory(ctx, req.UserId, req.GetFromDate(), req.GetToDate(), limit)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no active wallet") || strings.Contains(msg, "invalid user_id") {
			return &paymentpb.GetWaasTransactionHistoryResponse{Success: false, ErrorMessage: msg}, nil
		}
		if strings.Contains(msg, "9PSB WaaS") {
			return &paymentpb.GetWaasTransactionHistoryResponse{Success: false, ErrorMessage: msg}, nil
		}
		return &paymentpb.GetWaasTransactionHistoryResponse{Success: false, ErrorMessage: msg}, nil
	}
	out := make([]*paymentpb.WaasTransactionItem, 0, len(result.Data.Message))
	for _, t := range result.Data.Message {
		out = append(out, &paymentpb.WaasTransactionItem{
			TransactionDate:        t.TransactionDate,
			TransactionDateString:  t.TransactionDateString,
			Amount:                t.Amount,
			Narration:              t.Narration,
			Balance:               t.Balance,
			ReferenceId:           t.ReferenceID,
			Debit:                 t.Debit,
			Credit:                t.Credit,
			UniqueIdentifier:      t.UniqueIdentifier,
			IsReversed:            t.IsReversed,
		})
	}
	return &paymentpb.GetWaasTransactionHistoryResponse{
		Success:      true,
		Message:      result.Message,
		Transactions: out,
	}, nil
}

// GetWaasWalletStatus returns 9PSB WaaS wallet status for the given user's wallet (admin).
func (s *Server) GetWaasWalletStatus(ctx context.Context, req *paymentpb.GetWaasWalletStatusRequest) (*paymentpb.GetWaasWalletStatusResponse, error) {
	if req == nil || req.UserId == "" {
		return &paymentpb.GetWaasWalletStatusResponse{Success: false, ErrorMessage: "user_id required"}, nil
	}
	result, err := s.svc.GetWaasWalletStatus(ctx, req.UserId)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no active wallet") || strings.Contains(msg, "invalid user_id") {
			return &paymentpb.GetWaasWalletStatusResponse{Success: false, ErrorMessage: msg}, nil
		}
		if strings.Contains(msg, "9PSB WaaS") {
			return &paymentpb.GetWaasWalletStatusResponse{Success: false, ErrorMessage: msg}, nil
		}
		return &paymentpb.GetWaasWalletStatusResponse{Success: false, ErrorMessage: msg}, nil
	}
	return &paymentpb.GetWaasWalletStatusResponse{
		Success:      true,
		WalletStatus: result.Data.WalletStatus,
		ResponseCode: result.Data.ResponseCode,
	}, nil
}

// ChangeWalletStatus changes the user's wallet status via 9PSB WaaS (ACTIVE or SUSPENDED). Updates DB, sends audit and email.
func (s *Server) ChangeWalletStatus(ctx context.Context, req *paymentpb.ChangeWalletStatusRequest) (*paymentpb.ChangeWalletStatusResponse, error) {
	if req == nil || req.UserId == "" {
		return &paymentpb.ChangeWalletStatusResponse{Success: false, ErrorMessage: "user_id required"}, nil
	}
	if req.AccountStatus != "ACTIVE" && req.AccountStatus != "SUSPENDED" {
		return &paymentpb.ChangeWalletStatusResponse{Success: false, ErrorMessage: "account_status must be ACTIVE or SUSPENDED"}, nil
	}
	result, err := s.svc.ChangeWalletStatus(ctx, req.UserId, req.AccountStatus, req.InitiatedBy)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no wallet found") {
			return &paymentpb.ChangeWalletStatusResponse{Success: false, ErrorMessage: msg}, nil
		}
		if strings.Contains(msg, "9PSB WaaS") {
			return &paymentpb.ChangeWalletStatusResponse{Success: false, ErrorMessage: msg}, nil
		}
		return &paymentpb.ChangeWalletStatusResponse{Success: false, ErrorMessage: msg}, nil
	}
	return &paymentpb.ChangeWalletStatusResponse{Success: true, NewWalletStatus: result.NewWalletStatus}, nil
}

// SubmitWalletUpgrade submits a wallet tier upgrade to 9PSB (fetches KYC + images, multipart upload; audit + email via Kafka).
func (s *Server) SubmitWalletUpgrade(ctx context.Context, req *paymentpb.SubmitWalletUpgradeRequest) (*paymentpb.SubmitWalletUpgradeResponse, error) {
	if req == nil || req.UserId == "" {
		return &paymentpb.SubmitWalletUpgradeResponse{Success: false, ErrorMessage: "user_id required"}, nil
	}
	result, err := s.svc.SubmitWalletUpgrade(ctx, req.UserId, req.InitiatedBy)
	if err != nil {
		_ = s.svc.SendAuditLog(kafka.AuditLogParams{
			Service:  "payment",
			Action:   "admin_wallet_upgrade_failed",
			Entity:   "wallet",
			EntityID: req.UserId,
			UserID:   strPtr(req.InitiatedBy),
			Metadata: map[string]interface{}{"error": err.Error()},
		})
		return &paymentpb.SubmitWalletUpgradeResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &paymentpb.SubmitWalletUpgradeResponse{Success: true, Message: result.Message}, nil
}

// ListWalletUpgradeRequests returns wallet upgrade requests for admin (paginated).
func (s *Server) ListWalletUpgradeRequests(ctx context.Context, req *paymentpb.ListWalletUpgradeRequestsRequest) (*paymentpb.ListWalletUpgradeRequestsResponse, error) {
	limit := int(req.GetLimit())
	offset := int(req.GetOffset())
	list, err := s.svc.ListWalletUpgradeRequests(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*paymentpb.WalletUpgradeRequestItem, 0, len(list))
	for _, r := range list {
		out = append(out, &paymentpb.WalletUpgradeRequestItem{
			Id:               r.ID,
			WalletId:         r.WalletID,
			UserId:           r.UserID,
			UpgradeRef:       r.UpgradeRef,
			TierFrom:         r.TierFrom,
			TierTo:           r.TierTo,
			UpgradeMethod:    r.UpgradeMethod,
			InitiationStatus: r.InitiationStatus,
			FinalStatus:      r.FinalStatus,
			InitiatedBy:      r.InitiatedBy,
			SubmittedAt:      formatTimePtr(r.SubmittedAt),
			FinalizedAt:      formatTimePtr(r.FinalizedAt),
			CreatedAt:        r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return &paymentpb.ListWalletUpgradeRequestsResponse{Requests: out}, nil
}

// GetWalletUpgradeRequest returns one wallet upgrade request by id.
func (s *Server) GetWalletUpgradeRequest(ctx context.Context, req *paymentpb.GetWalletUpgradeRequestRequest) (*paymentpb.GetWalletUpgradeRequestResponse, error) {
	if req == nil || req.Id == "" {
		return &paymentpb.GetWalletUpgradeRequestResponse{Found: false}, nil
	}
	detail, err := s.svc.GetWalletUpgradeRequestByID(ctx, req.Id)
	if err != nil || detail == nil {
		return &paymentpb.GetWalletUpgradeRequestResponse{Found: false}, nil
	}
	r := &detail.WalletUpgradeRequestRow
	item := &paymentpb.WalletUpgradeRequestItem{
		Id:               r.ID,
		WalletId:         r.WalletID,
		UserId:           r.UserID,
		UpgradeRef:       r.UpgradeRef,
		TierFrom:         r.TierFrom,
		TierTo:           r.TierTo,
		UpgradeMethod:    r.UpgradeMethod,
		InitiationStatus: r.InitiationStatus,
		FinalStatus:      r.FinalStatus,
		InitiatedBy:      r.InitiatedBy,
		SubmittedAt:      formatTimePtr(r.SubmittedAt),
		FinalizedAt:      formatTimePtr(r.FinalizedAt),
		CreatedAt:        r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	return &paymentpb.GetWalletUpgradeRequestResponse{
		Found:               true,
		Item:                item,
		RequestPayloadJson:  detail.RequestPayloadJSON,
		ResponsePayloadJson: detail.ResponsePayloadJSON,
	}, nil
}

// GetWalletUpgradeStatusByUserID returns wallet upgrade status from 9PSB upgrade_status API (source of truth) plus optional latest request row.
func (s *Server) GetWalletUpgradeStatusByUserID(ctx context.Context, req *paymentpb.GetWalletUpgradeStatusByUserIDRequest) (*paymentpb.GetWalletUpgradeStatusByUserIDResponse, error) {
	if req == nil || req.UserId == "" {
		return &paymentpb.GetWalletUpgradeStatusByUserIDResponse{}, nil
	}
	result, err := s.svc.GetWalletUpgradeStatus(ctx, req.UserId)
	if err != nil {
		return nil, err
	}
	out := &paymentpb.GetWalletUpgradeStatusByUserIDResponse{HasWallet: result.HasWallet}
	if result.UpgradeStatus != nil {
		out.UpgradeStatus = &paymentpb.UpgradeStatusFrom9PSB{
			Status:      result.UpgradeStatus.Status,
			Message:     result.UpgradeStatus.Message,
			DataMessage: result.UpgradeStatus.Data.Message,
			DataStatus:  result.UpgradeStatus.Data.Status,
		}
	}
	if result.Latest != nil {
		r := result.Latest
		out.Latest = &paymentpb.WalletUpgradeRequestItem{
			Id:               r.ID,
			WalletId:         r.WalletID,
			UserId:           r.UserID,
			UpgradeRef:       r.UpgradeRef,
			TierFrom:         r.TierFrom,
			TierTo:           r.TierTo,
			UpgradeMethod:    r.UpgradeMethod,
			InitiationStatus: r.InitiationStatus,
			FinalStatus:      r.FinalStatus,
			InitiatedBy:      r.InitiatedBy,
			SubmittedAt:      formatTimePtr(r.SubmittedAt),
			FinalizedAt:      formatTimePtr(r.FinalizedAt),
			CreatedAt:        r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return out, nil
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02T15:04:05Z07:00")
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
