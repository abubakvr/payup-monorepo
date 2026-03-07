package clients

import (
	"context"

	userpb "github.com/abubakvr/payup-backend/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserClient calls the user service gRPC (GetUserForKYC, ValidateTransfer for payment).
type UserClient struct {
	kycClient     userpb.UserServiceForKYCClient
	paymentClient userpb.UserServiceForPaymentClient
	conn          *grpc.ClientConn
}

// NewUserClient dials the user service and returns a client. Call Close when done.
func NewUserClient(addr string) (*UserClient, error) {
	if addr == "" {
		return nil, nil
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &UserClient{
		kycClient:     userpb.NewUserServiceForKYCClient(conn),
		paymentClient: userpb.NewUserServiceForPaymentClient(conn),
		conn:          conn,
	}, nil
}

func (c *UserClient) Close() error {
	if c != nil && c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetUserForKYC returns user email and names. Use for wallet open when KYC has no email.
func (c *UserClient) GetUserForKYC(ctx context.Context, userID string) (*userpb.GetUserForKYCResponse, error) {
	if c == nil || c.kycClient == nil {
		return nil, nil
	}
	return c.kycClient.GetUserForKYC(ctx, &userpb.GetUserForKYCRequest{UserId: userID})
}

// ValidateTransfer checks user is allowed to transfer (PIN, restricted, transfers paused). Daily/monthly limits checked by payment.
func (c *UserClient) ValidateTransfer(ctx context.Context, userID string, amount float64, pin string) (*userpb.ValidateTransferResponse, error) {
	if c == nil || c.paymentClient == nil {
		return &userpb.ValidateTransferResponse{Allowed: false, Message: "user service not configured"}, nil
	}
	return c.paymentClient.ValidateTransfer(ctx, &userpb.ValidateTransferRequest{UserId: userID, Amount: amount, Pin: pin})
}
