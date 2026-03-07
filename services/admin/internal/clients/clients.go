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
