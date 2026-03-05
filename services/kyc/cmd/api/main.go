package main

import (
	"context"
	"log"
	"net"
	"os"
	"strings"

	kycpb "github.com/abubakvr/payup-backend/proto/kyc"
	"github.com/abubakvr/payup-backend/services/kyc/internal/clients"
	"github.com/abubakvr/payup-backend/services/kyc/internal/config"
	"github.com/abubakvr/payup-backend/services/kyc/internal/controller"
	"github.com/abubakvr/payup-backend/services/kyc/internal/dojah"
	kycgrpc "github.com/abubakvr/payup-backend/services/kyc/internal/grpc"
	"github.com/abubakvr/payup-backend/services/kyc/internal/kafka"
	"github.com/abubakvr/payup-backend/services/kyc/internal/repository"
	"github.com/abubakvr/payup-backend/services/kyc/internal/router"
	"github.com/abubakvr/payup-backend/services/kyc/internal/service"
	"github.com/abubakvr/payup-backend/services/kyc/internal/storage"
	"github.com/abubakvr/payup-backend/services/kyc/internal/worker"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.LoadConfig()

	db, err := config.OpenDB(cfg)
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}
	defer db.Close()

	userClient, err := clients.NewUserClient(cfg.UserServiceGrpcAddr)
	if err != nil {
		log.Fatalf("failed to connect to user service gRPC: %v", err)
	}
	defer userClient.Close()

	brokers := strings.Split(cfg.KafkaBroker, ",")
	for i, b := range brokers {
		brokers[i] = strings.TrimSpace(b)
	}
	auditProducer := kafka.NewAuditProducer(brokers)
	notifier := kafka.NewNotificationProducer(brokers)
	dojahConfig := dojah.DefaultConfig()
	s3Cfg := storage.LoadS3ConfigFromEnv()
	var selfieUploader service.SelfieUploader
	if s3Cfg.Bucket != "" {
		if u, err := storage.NewS3Uploader(s3Cfg); err != nil {
			log.Printf("WARN: S3 uploader init failed (utility bill / proof of address / identity / selfie uploads disabled): %v", err)
		} else {
			selfieUploader = u
			log.Printf("S3 uploader configured: bucket=%s region=%s", s3Cfg.Bucket, s3Cfg.Region)
		}
	} else {
		log.Printf("WARN: KYC_S3_BUCKET not set; file uploads (utility bill, proof of address, identity, selfie) disabled")
	}

	var uploadPool *worker.Pool
	if cfg.UploadWorkers > 0 {
		uploadPool = worker.NewPool(cfg.UploadWorkers, cfg.UploadQueueSize)
		uploadPool.Start(context.Background())
		defer uploadPool.Stop()
		log.Printf("Upload worker pool started: workers=%d queue=%d", cfg.UploadWorkers, cfg.UploadQueueSize)
	}

	repo := repository.NewKYCRepository(db, cfg.EncryptionKey)
	svc := service.NewKYCService(repo, userClient, auditProducer, notifier, dojahConfig, selfieUploader, uploadPool)
	ctrl := controller.NewKYCController(svc)

	r := router.SetupRouter(cfg, ctrl)

	consumer := kafka.NewConsumer(brokers)
	go consumer.Start()

	// gRPC server for Admin service (GetFullKYCForAdmin, GetKYCStatus)
	grpcPort := "9002"
	if p := strings.TrimSpace(os.Getenv("KYC_GRPC_PORT")); p != "" {
		grpcPort = p
	}
	go func() {
		lis, err := net.Listen("tcp", ":"+grpcPort)
		if err != nil {
			log.Printf("kyc gRPC listen: %v", err)
			return
		}
		srv := grpclib.NewServer()
		kycpb.RegisterKYCServiceServer(srv, kycgrpc.NewKYCAdminServer(svc))
		reflection.Register(srv)
		log.Printf("KYC gRPC listening on port %s", grpcPort)
		if err := srv.Serve(lis); err != nil {
			log.Printf("kyc gRPC serve: %v", err)
		}
	}()

	log.Printf("KYC service running on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
