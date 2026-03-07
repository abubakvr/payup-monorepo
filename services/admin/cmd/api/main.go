package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/abubakvr/payup-backend/services/admin/internal/config"
	"github.com/abubakvr/payup-backend/services/admin/internal/controller"
	"github.com/abubakvr/payup-backend/services/admin/internal/repository"
	"github.com/abubakvr/payup-backend/services/admin/internal/clients"
	"github.com/abubakvr/payup-backend/services/admin/internal/kafka"
	"github.com/abubakvr/payup-backend/services/admin/internal/router"
	"github.com/abubakvr/payup-backend/services/admin/internal/service"
)

func main() {
	cfg := config.LoadConfig()
	db, err := config.OpenDB(cfg)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	repo := repository.NewAdminRepository(db)
	svc := service.NewAdminService(repo)

	// gRPC clients for portal (users, KYC, audits)
	var userClient *clients.UserAdminClient
	var kycClient *clients.KYCAdminClient
	var auditClient *clients.AuditAdminClient
	if u, err := clients.NewUserAdminClient(cfg.UserServiceGrpcAddr); err != nil {
		log.Printf("admin: user gRPC client: %v", err)
	} else {
		userClient = u
		defer userClient.Close()
	}
	if k, err := clients.NewKYCAdminClient(cfg.KYCServiceGrpcAddr); err != nil {
		log.Printf("admin: kyc gRPC client: %v", err)
	} else {
		kycClient = k
		defer kycClient.Close()
	}
	if a, err := clients.NewAuditAdminClient(cfg.AuditServiceGrpcAddr); err != nil {
		log.Printf("admin: audit gRPC client: %v", err)
	} else {
		auditClient = a
		defer auditClient.Close()
	}
	var paymentClient *clients.PaymentAdminClient
	if cfg.PaymentServiceGrpcAddr != "" {
		if p, err := clients.NewPaymentAdminClient(cfg.PaymentServiceGrpcAddr); err != nil {
			log.Printf("admin: payment gRPC client: %v", err)
		} else {
			paymentClient = p
			defer paymentClient.Close()
		}
	}

	var auditProducer *kafka.AuditProducer
	var notificationProducer *kafka.NotificationProducer
	if cfg.KafkaBroker != "" {
		brokers := strings.Split(cfg.KafkaBroker, ",")
		for i := range brokers {
			brokers[i] = strings.TrimSpace(brokers[i])
		}
		auditProducer = kafka.NewAuditProducer(brokers)
		notificationProducer = kafka.NewNotificationProducer(brokers)
	}

	// Bootstrap first super admin from env if no admins exist
	if created, err := svc.BootstrapSuperAdmin(
		context.TODO(),
		cfg.BootstrapEmail,
		cfg.BootstrapPassword,
		cfg.BootstrapFirstName,
		cfg.BootstrapLastName,
	); err != nil {
		log.Printf("bootstrap super admin: %v", err)
	} else if created {
		log.Printf("Super admin created from ADMIN_BOOTSTRAP_* env")
	}

	ctrl := controller.NewAdminController(svc, userClient, kycClient, auditClient, paymentClient, auditProducer, notificationProducer, cfg.AdminPortalURL, cfg.KYCAdminAPIKey)
	r := router.Setup(ctrl)

	addr := ":" + cfg.Port
	log.Printf("Admin service listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
