package main

import (
	"log"
	"strings"

	"github.com/abubakvr/payup-backend/services/kyc/internal/clients"
	"github.com/abubakvr/payup-backend/services/kyc/internal/config"
	"github.com/abubakvr/payup-backend/services/kyc/internal/controller"
	"github.com/abubakvr/payup-backend/services/kyc/internal/dojah"
	"github.com/abubakvr/payup-backend/services/kyc/internal/kafka"
	"github.com/abubakvr/payup-backend/services/kyc/internal/repository"
	"github.com/abubakvr/payup-backend/services/kyc/internal/router"
	"github.com/abubakvr/payup-backend/services/kyc/internal/service"
	"github.com/abubakvr/payup-backend/services/kyc/internal/storage"
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

	repo := repository.NewKYCRepository(db, cfg.EncryptionKey)
	svc := service.NewKYCService(repo, userClient, auditProducer, notifier, dojahConfig, selfieUploader)
	ctrl := controller.NewKYCController(svc)

	r := router.SetupRouter(cfg, ctrl)

	consumer := kafka.NewConsumer(brokers)
	go consumer.Start()

	log.Printf("KYC service running on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
