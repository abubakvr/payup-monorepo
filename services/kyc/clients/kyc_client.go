package clients

import (
	"context"
	"time"

	"google.golang.org/grpc"

	kycpb "github.com/abubakvr/payup-backend/services/kyc/proto/kyc"
)

type KYCClient struct {
	client kycpb.KYCServiceClient
}

func NewKYCClient() (*KYCClient, error) {
	conn, err := grpc.Dial(
		"0.0.0.0:9002",
		grpc.WithInsecure(),
		grpc.WithTimeout(3*time.Second),
	)

	if err != nil {
		return nil, err
	}

	return &KYCClient{
		client: kycpb.NewKYCServiceClient(conn),
	}, nil
}

func (k *KYCClient) GetStatus(ctx context.Context, userID string) (string, error) {
	res, err := k.client.GetKYCStatus(ctx, &kycpb.GetKYCStatusRequest{
		UserId: userID,
	})

	if err != nil {
		return "", err
	}

	return res.Status, nil
}
