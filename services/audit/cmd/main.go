package main

import (
	"log"
	"net"
	"net/http"
	"os"

	"github.com/abubakvr/payup-backend/services/audit/internal/config"
	"github.com/abubakvr/payup-backend/services/audit/internal/controller"
	auditgrpc "github.com/abubakvr/payup-backend/services/audit/internal/grpc"
	auditpb "github.com/abubakvr/payup-backend/proto/audit"
	"github.com/abubakvr/payup-backend/services/audit/internal/kafka"
	"github.com/abubakvr/payup-backend/services/audit/internal/repository"
	"github.com/abubakvr/payup-backend/services/audit/internal/router"
	"github.com/abubakvr/payup-backend/services/audit/internal/service"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	cfg := config.LoadConfig()
	db, err := config.OpenDB(cfg)
	if err != nil {
		log.Fatalf("audit: db: %v", err)
	}
	defer db.Close()

	repo := repository.NewAuditRepository(db)
	svc := service.NewAuditService(repo)
	ctrl := controller.NewAuditController(svc)
	r := router.SetupRouter(ctrl)

	consumer := kafka.NewConsumer(
		[]string{os.Getenv("KAFKA_BROKER")},
		svc,
	)
	go consumer.Start()

	// gRPC server for Admin service (ListAllAudits, GetUserAudits)
	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GrpcPort)
		if err != nil {
			log.Printf("audit gRPC listen: %v", err)
			return
		}
		srv := grpclib.NewServer()
		auditpb.RegisterAuditServiceServer(srv, auditgrpc.NewAuditServer(svc))
		reflection.Register(srv)
		log.Printf("Audit gRPC listening on port %s", cfg.GrpcPort)
		if err := srv.Serve(lis); err != nil {
			log.Printf("audit gRPC serve: %v", err)
		}
	}()

	log.Printf("Audit service running on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
