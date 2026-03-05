// cmd/api/main.go
package main

import (
	"log"
	"net"
	"time"

	"github.com/abubakvr/payup-backend/services/user/internal/config"
	"github.com/abubakvr/payup-backend/services/user/internal/controller"
	"github.com/abubakvr/payup-backend/services/user/internal/grpc"
	"github.com/abubakvr/payup-backend/services/user/internal/kafka"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
	"github.com/abubakvr/payup-backend/services/user/internal/router"
	"github.com/abubakvr/payup-backend/services/user/internal/service"
	"github.com/abubakvr/payup-backend/services/user/redis"
	userpb "github.com/abubakvr/payup-backend/proto/user"
	grpclib "google.golang.org/grpc"
)

func main() {
	cfg := config.LoadConfig()
	db, err := config.OpenDB(cfg)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer db.Close()

	tokenGen := service.NewTokenGenerator()
	userRepo := repository.NewUserRepository(db, tokenGen)
	producer := kafka.NewProducer([]string{cfg.KafkaBroker})
	userExistsTTL := time.Duration(cfg.UserExistsCacheTTLSeconds) * time.Second
	userSvc := service.NewUserService(userRepo, tokenGen, producer, cfg.EmailVerificationBaseURL, cfg.PasswordResetBaseURL, userExistsTTL)
	userCtrl := controller.NewUserController(userSvc)
	r := router.SetupRouter(cfg, userCtrl)

	redis.InitRedis()

	// gRPC server for KYC service (GetUserForKYC)
	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GrpcPort)
		if err != nil {
			log.Fatalf("grpc listen: %v", err)
		}
		srv := grpclib.NewServer()
		userpb.RegisterUserServiceForKYCServer(srv, grpc.NewKYCUserServer(userRepo))
		userpb.RegisterUserServiceForAdminServer(srv, grpc.NewAdminUserServer(userRepo, userSvc))
		log.Printf("User gRPC (KYC) listening on port %s", cfg.GrpcPort)
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("grpc serve: %v", err)
		}
	}()

	log.Printf("User service running on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start user service: %v", err)
	}
}
