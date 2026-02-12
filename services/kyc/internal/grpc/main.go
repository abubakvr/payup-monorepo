package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	kycpb "github.com/abubakvr/payup-backend/services/kyc/proto/kyc"
)

func main() {
	lis, err := net.Listen("tcp", "0.0.0.0:9002")
	if err != nil {
		log.Fatal(err)
	}

	s := grpc.NewServer()
	kycpb.RegisterKYCServiceServer(s, NewServer())
	reflection.Register(s)

	log.Println("KYC service running on port 9002")
	s.Serve(lis)
}
