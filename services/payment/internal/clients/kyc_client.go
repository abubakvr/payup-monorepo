package clients

import (
	"context"

	kycpb "github.com/abubakvr/payup-backend/proto/kyc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// KYCClient calls the KYC service gRPC (GetKYCForWallet).
type KYCClient struct {
	client kycpb.KYCServiceClient
	conn   *grpc.ClientConn
}

// NewKYCClient dials the KYC service and returns a client. Call Close when done.
func NewKYCClient(addr string) (*KYCClient, error) {
	if addr == "" {
		return nil, nil
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &KYCClient{
		client: kycpb.NewKYCServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *KYCClient) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetKYCForWallet returns full KYC data for 9PSB open_wallet. Returns nil if not found or client is nil.
func (c *KYCClient) GetKYCForWallet(ctx context.Context, userID string) (*kycpb.GetKYCForWalletResponse, error) {
	if c == nil || c.client == nil {
		return nil, nil
	}
	return c.client.GetKYCForWallet(ctx, &kycpb.GetKYCForWalletRequest{UserId: userID})
}

// GetKYCForWalletUpgrade returns KYC data and image bytes for 9PSB wallet_upgrade_file_upload. Returns nil if not found or client is nil.
func (c *KYCClient) GetKYCForWalletUpgrade(ctx context.Context, userID string) (*kycpb.GetKYCForWalletUpgradeResponse, error) {
	if c == nil || c.client == nil {
		return nil, nil
	}
	return c.client.GetKYCForWalletUpgrade(ctx, &kycpb.GetKYCForWalletUpgradeRequest{UserId: userID})
}
