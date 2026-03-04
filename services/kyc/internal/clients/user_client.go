package clients

import (
	"context"
	"log"

	userpb "github.com/abubakvr/payup-backend/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserClient calls the user service gRPC (GetUserForKYC).
type UserClient struct {
	client userpb.UserServiceForKYCClient
	conn   *grpc.ClientConn
}

// NewUserClient dials the user service and returns a client. Call Close when done.
func NewUserClient(addr string) (*UserClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &UserClient{
		client: userpb.NewUserServiceForKYCClient(conn),
		conn:   conn,
	}, nil
}

func (c *UserClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetUserForKYC returns user details if found. Use response.Found to check.
func (c *UserClient) GetUserForKYC(ctx context.Context, userID string) (*userpb.GetUserForKYCResponse, error) {
	resp, err := c.client.GetUserForKYC(ctx, &userpb.GetUserForKYCRequest{UserId: userID})
	if err != nil {
		log.Printf("kyc: user gRPC GetUserForKYC error: %v", err)
		return nil, err
	}
	return resp, nil
}
