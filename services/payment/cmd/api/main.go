package main

import (
	"log"
	"net"
	"os"
	"strings"

	paymentpb "github.com/abubakvr/payup-backend/proto/payment"
	"github.com/abubakvr/payup-backend/services/payment/internal/config"
	"github.com/abubakvr/payup-backend/services/payment/internal/controller"
	"github.com/abubakvr/payup-backend/services/payment/internal/clients"
	paymentgrpc "github.com/abubakvr/payup-backend/services/payment/internal/grpc"
	"github.com/abubakvr/payup-backend/services/payment/internal/kafka"
	"github.com/abubakvr/payup-backend/services/payment/internal/psb"
	"github.com/abubakvr/payup-backend/services/payment/internal/repository"
	"github.com/abubakvr/payup-backend/services/payment/internal/router"
	"github.com/abubakvr/payup-backend/services/payment/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.Load()

	db, err := config.OpenDB(cfg)
	if err != nil {
		log.Fatalf("payment: failed to open DB: %v", err)
	}
	defer db.Close()

	brokers := strings.Split(cfg.KafkaBroker, ",")
	for i, b := range brokers {
		brokers[i] = strings.TrimSpace(b)
	}
	producer := kafka.NewProducer(brokers)
	if producer == nil {
		log.Printf("payment: Kafka brokers empty; audit and notification sending disabled")
	}

	repo := repository.NewPaymentRepository(db)
	walletRepo := repository.NewWalletRepository(db, cfg.EncryptionKey)
	transactionRepo := repository.NewTransactionRepository(db, cfg.EncryptionKey)
	authRepo := repository.NewAuthTokenRepository(db, cfg.EncryptionKey)

	var kycClient *clients.KYCClient
	if cfg.KYCServiceGrpcAddr != "" {
		kycClient, err = clients.NewKYCClient(cfg.KYCServiceGrpcAddr)
		if err != nil {
			log.Fatalf("payment: KYC gRPC client: %v", err)
		}
		defer kycClient.Close()
	}
	var userClient *clients.UserClient
	if cfg.UserServiceGrpcAddr != "" {
		userClient, err = clients.NewUserClient(cfg.UserServiceGrpcAddr)
		if err != nil {
			log.Fatalf("payment: User gRPC client: %v", err)
		}
		defer userClient.Close()
	}

	var psbProvider *psb.TokenProvider
	if cfg.PsbBaseURL != "" && cfg.PsbClientID != "" && cfg.EncryptionKey != "" {
		psbProvider = psb.NewTokenProvider(cfg.PsbBaseURL, cfg.PsbBaseURL2, cfg.PsbUsername, cfg.PsbPassword, cfg.PsbClientID, cfg.PsbClientSecret, authRepo)
	} else {
		log.Printf("payment: 9PSB or encryption key not set; wallet creation disabled")
	}

	svc := service.NewPaymentService(repo, walletRepo, transactionRepo, producer, producer, kycClient, userClient, psbProvider)
	ctrl := controller.NewPaymentController(svc, cfg)

	r := router.Setup(ctrl)

	// gRPC server
	grpcPort := cfg.GrpcPort
	if p := strings.TrimSpace(os.Getenv("PAYMENT_GRPC_PORT")); p != "" {
		grpcPort = p
	}
	go func() {
		lis, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			log.Fatalf("payment gRPC listen: %v", err)
		}
		srv := grpc.NewServer()
		paymentpb.RegisterPaymentServiceServer(srv, paymentgrpc.NewServer(svc))
		reflection.Register(srv)
		log.Printf("Payment gRPC listening on port %s", grpcPort)
		if err := srv.Serve(lis); err != nil {
			log.Printf("payment gRPC serve: %v", err)
		}
	}()

	log.Printf("Payment service HTTP running on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
