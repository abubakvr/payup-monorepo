package clients

import (
	"context"
	"log"

	auditpb "github.com/abubakvr/payup-backend/proto/audit"
	kycpb "github.com/abubakvr/payup-backend/proto/kyc"
	paymentpb "github.com/abubakvr/payup-backend/proto/payment"
	userpb "github.com/abubakvr/payup-backend/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserAdminClient calls user service gRPC for admin (ListUsers, GetUserForAdmin).
type UserAdminClient struct {
	client userpb.UserServiceForAdminClient
	conn   *grpc.ClientConn
}

func NewUserAdminClient(addr string) (*UserAdminClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &UserAdminClient{
		client: userpb.NewUserServiceForAdminClient(conn),
		conn:   conn,
	}, nil
}

func (c *UserAdminClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *UserAdminClient) ListUsers(ctx context.Context, limit, offset int32) (*userpb.ListUsersResponse, error) {
	resp, err := c.client.ListUsers(ctx, &userpb.ListUsersRequest{Limit: limit, Offset: offset})
	if err != nil {
		log.Printf("admin: user gRPC ListUsers: %v", err)
		return nil, err
	}
	return resp, nil
}

func (c *UserAdminClient) GetUserForAdmin(ctx context.Context, userID string) (*userpb.GetUserForAdminResponse, error) {
	resp, err := c.client.GetUserForAdmin(ctx, &userpb.GetUserForAdminRequest{UserId: userID})
	if err != nil {
		log.Printf("admin: user gRPC GetUserForAdmin: %v", err)
		return nil, err
	}
	return resp, nil
}

func (c *UserAdminClient) SetUserRestricted(ctx context.Context, userID string, restricted bool) (*userpb.SetUserRestrictedResponse, error) {
	resp, err := c.client.SetUserRestricted(ctx, &userpb.SetUserRestrictedRequest{UserId: userID, Restricted: restricted})
	if err != nil {
		log.Printf("admin: user gRPC SetUserRestricted: %v", err)
		return nil, err
	}
	return resp, nil
}

// KYCAdminClient calls KYC service gRPC for admin (GetFullKYCForAdmin).
type KYCAdminClient struct {
	client kycpb.KYCServiceClient
	conn   *grpc.ClientConn
}

func NewKYCAdminClient(addr string) (*KYCAdminClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &KYCAdminClient{
		client: kycpb.NewKYCServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *KYCAdminClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *KYCAdminClient) GetFullKYCForAdmin(ctx context.Context, userID string) (*kycpb.GetFullKYCForAdminResponse, error) {
	resp, err := c.client.GetFullKYCForAdmin(ctx, &kycpb.GetFullKYCForAdminRequest{UserId: userID})
	if err != nil {
		log.Printf("admin: kyc gRPC GetFullKYCForAdmin: %v", err)
		return nil, err
	}
	return resp, nil
}

// CountProfiles returns the number of KYC profiles (optionally filtered by status and/or kyc_level). Used for kyc-list total.
func (c *KYCAdminClient) CountProfiles(ctx context.Context, status string, kycLevel *int32) (int64, error) {
	req := &kycpb.CountProfilesRequest{Status: status}
	if kycLevel != nil {
		req.KycLevel = kycLevel
	}
	resp, err := c.client.CountProfiles(ctx, req)
	if err != nil {
		log.Printf("admin: kyc gRPC CountProfiles: %v", err)
		return 0, err
	}
	return resp.Count, nil
}

// ApproveKYC sets the user's KYC status to approved and triggers success email. Used after wallet creation.
func (c *KYCAdminClient) ApproveKYC(ctx context.Context, userID string) (*kycpb.ApproveKYCResponse, error) {
	resp, err := c.client.ApproveKYC(ctx, &kycpb.ApproveKYCRequest{UserId: userID})
	if err != nil {
		log.Printf("admin: kyc gRPC ApproveKYC: %v", err)
		return nil, err
	}
	return resp, nil
}

// AuditAdminClient calls audit service gRPC for admin (ListAllAudits, GetUserAudits).
type AuditAdminClient struct {
	client auditpb.AuditServiceClient
	conn   *grpc.ClientConn
}

func NewAuditAdminClient(addr string) (*AuditAdminClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &AuditAdminClient{
		client: auditpb.NewAuditServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *AuditAdminClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *AuditAdminClient) ListAllAudits(ctx context.Context, limit, offset int32) (*auditpb.AuditResponse, error) {
	resp, err := c.client.ListAllAudits(ctx, &auditpb.ListAllAuditsRequest{Limit: limit, Offset: offset})
	if err != nil {
		log.Printf("admin: audit gRPC ListAllAudits: %v", err)
		return nil, err
	}
	return resp, nil
}

func (c *AuditAdminClient) GetUserAudits(ctx context.Context, userID string, limit, offset int32) (*auditpb.AuditResponse, error) {
	resp, err := c.client.GetUserAudits(ctx, &auditpb.UserAuditRequest{UserId: userID, Limit: limit, Offset: offset})
	if err != nil {
		log.Printf("admin: audit gRPC GetUserAudits: %v", err)
		return nil, err
	}
	return resp, nil
}

// PaymentAdminClient calls payment service gRPC for wallet creation (CreateWallet).
type PaymentAdminClient struct {
	client paymentpb.PaymentServiceClient
	conn   *grpc.ClientConn
}

func NewPaymentAdminClient(addr string) (*PaymentAdminClient, error) {
	if addr == "" {
		return nil, nil
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &PaymentAdminClient{
		client: paymentpb.NewPaymentServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *PaymentAdminClient) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *PaymentAdminClient) CreateWallet(ctx context.Context, userID string) (*paymentpb.CreateWalletResponse, error) {
	if c == nil || c.client == nil {
		return &paymentpb.CreateWalletResponse{Success: false, ErrorMessage: "payment client not configured"}, nil
	}
	resp, err := c.client.CreateWallet(ctx, &paymentpb.CreateWalletRequest{UserId: userID})
	if err != nil {
		log.Printf("admin: payment gRPC CreateWallet: %v", err)
		return nil, err
	}
	return resp, nil
}

// ListWallets returns all wallets with details for admin view (paginated). limit default 50 max 100.
func (c *PaymentAdminClient) ListWallets(ctx context.Context, limit, offset int32) (*paymentpb.ListWalletsResponse, error) {
	if c == nil || c.client == nil {
		return nil, nil
	}
	resp, err := c.client.ListWallets(ctx, &paymentpb.ListWalletsRequest{Limit: limit, Offset: offset})
	if err != nil {
		log.Printf("admin: payment gRPC ListWallets: %v", err)
		return nil, err
	}
	return resp, nil
}

// DebitCreditWallet performs an internal debit or credit on a user's wallet (airtime, data, electricity, DSTV, etc.). Saves to transactions and ledger.
func (c *PaymentAdminClient) DebitCreditWallet(ctx context.Context, userID string, amount float64, isCredit bool, narration string, initiatedBy string) (*paymentpb.DebitCreditWalletResponse, error) {
	if c == nil || c.client == nil {
		return &paymentpb.DebitCreditWalletResponse{Success: false, ErrorMessage: "payment client not configured"}, nil
	}
	resp, err := c.client.DebitCreditWallet(ctx, &paymentpb.DebitCreditWalletRequest{
		UserId:      userID,
		Amount:      amount,
		IsCredit:    isCredit,
		Narration:   narration,
		InitiatedBy: initiatedBy,
	})
	if err != nil {
		log.Printf("admin: payment gRPC DebitCreditWallet: %v", err)
		return nil, err
	}
	return resp, nil
}

// GetWaasTransactionHistory returns 9PSB WaaS transaction history for the given user's wallet. Date range max 31 days.
func (c *PaymentAdminClient) GetWaasTransactionHistory(ctx context.Context, userID, fromDate, toDate string, limit int32) (*paymentpb.GetWaasTransactionHistoryResponse, error) {
	if c == nil || c.client == nil {
		return &paymentpb.GetWaasTransactionHistoryResponse{Success: false, ErrorMessage: "payment client not configured"}, nil
	}
	resp, err := c.client.GetWaasTransactionHistory(ctx, &paymentpb.GetWaasTransactionHistoryRequest{
		UserId:   userID,
		FromDate: fromDate,
		ToDate:   toDate,
		Limit:    limit,
	})
	if err != nil {
		log.Printf("admin: payment gRPC GetWaasTransactionHistory: %v", err)
		return nil, err
	}
	return resp, nil
}

// GetWaasWalletStatus returns 9PSB WaaS wallet status for the given user's wallet.
func (c *PaymentAdminClient) GetWaasWalletStatus(ctx context.Context, userID string) (*paymentpb.GetWaasWalletStatusResponse, error) {
	if c == nil || c.client == nil {
		return &paymentpb.GetWaasWalletStatusResponse{Success: false, ErrorMessage: "payment client not configured"}, nil
	}
	resp, err := c.client.GetWaasWalletStatus(ctx, &paymentpb.GetWaasWalletStatusRequest{UserId: userID})
	if err != nil {
		log.Printf("admin: payment gRPC GetWaasWalletStatus: %v", err)
		return nil, err
	}
	return resp, nil
}

// ChangeWalletStatus changes the user's wallet status via 9PSB (ACTIVE or SUSPENDED). Payment service updates DB and sends audit + email.
func (c *PaymentAdminClient) ChangeWalletStatus(ctx context.Context, userID, accountStatus, initiatedBy string) (*paymentpb.ChangeWalletStatusResponse, error) {
	if c == nil || c.client == nil {
		return &paymentpb.ChangeWalletStatusResponse{Success: false, ErrorMessage: "payment client not configured"}, nil
	}
	resp, err := c.client.ChangeWalletStatus(ctx, &paymentpb.ChangeWalletStatusRequest{
		UserId:       userID,
		AccountStatus: accountStatus,
		InitiatedBy:  initiatedBy,
	})
	if err != nil {
		log.Printf("admin: payment gRPC ChangeWalletStatus: %v", err)
		return nil, err
	}
	return resp, nil
}

// SubmitWalletUpgrade submits a wallet tier upgrade to 9PSB (payment fetches KYC + images, sends multipart; audit + email via Kafka).
func (c *PaymentAdminClient) SubmitWalletUpgrade(ctx context.Context, userID, initiatedBy string) (*paymentpb.SubmitWalletUpgradeResponse, error) {
	if c == nil || c.client == nil {
		return &paymentpb.SubmitWalletUpgradeResponse{Success: false, ErrorMessage: "payment client not configured"}, nil
	}
	resp, err := c.client.SubmitWalletUpgrade(ctx, &paymentpb.SubmitWalletUpgradeRequest{
		UserId:       userID,
		InitiatedBy:  initiatedBy,
	})
	if err != nil {
		log.Printf("admin: payment gRPC SubmitWalletUpgrade: %v", err)
		return nil, err
	}
	return resp, nil
}

// ListWalletUpgradeRequests returns wallet upgrade requests for admin (paginated).
func (c *PaymentAdminClient) ListWalletUpgradeRequests(ctx context.Context, limit, offset int32) (*paymentpb.ListWalletUpgradeRequestsResponse, error) {
	if c == nil || c.client == nil {
		return &paymentpb.ListWalletUpgradeRequestsResponse{}, nil
	}
	return c.client.ListWalletUpgradeRequests(ctx, &paymentpb.ListWalletUpgradeRequestsRequest{Limit: limit, Offset: offset})
}

// GetWalletUpgradeRequest returns one wallet upgrade request by id.
func (c *PaymentAdminClient) GetWalletUpgradeRequest(ctx context.Context, id string) (*paymentpb.GetWalletUpgradeRequestResponse, error) {
	if c == nil || c.client == nil {
		return &paymentpb.GetWalletUpgradeRequestResponse{Found: false}, nil
	}
	return c.client.GetWalletUpgradeRequest(ctx, &paymentpb.GetWalletUpgradeRequestRequest{Id: id})
}

// GetWalletUpgradeStatusByUserID returns wallet upgrade status from 9PSB upgrade_status API for a user.
func (c *PaymentAdminClient) GetWalletUpgradeStatusByUserID(ctx context.Context, userID string) (*paymentpb.GetWalletUpgradeStatusByUserIDResponse, error) {
	if c == nil || c.client == nil {
		return &paymentpb.GetWalletUpgradeStatusByUserIDResponse{}, nil
	}
	return c.client.GetWalletUpgradeStatusByUserID(ctx, &paymentpb.GetWalletUpgradeStatusByUserIDRequest{UserId: userID})
}
