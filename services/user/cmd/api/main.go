// cmd/api/main.go
package main

import (
	"log"

	"github.com/abubakvr/payup-backend/services/user/internal/config"
	"github.com/abubakvr/payup-backend/services/user/internal/controller"
	"github.com/abubakvr/payup-backend/services/user/internal/kafka"
	"github.com/abubakvr/payup-backend/services/user/internal/repository"
	"github.com/abubakvr/payup-backend/services/user/internal/router"
	"github.com/abubakvr/payup-backend/services/user/internal/service"
	"github.com/abubakvr/payup-backend/services/user/redis"
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
	producer := kafka.NewProducer([]string{"redpanda:9092"})
	userSvc := service.NewUserService(userRepo, tokenGen, producer)
	userCtrl := controller.NewUserController(userSvc)
	r := router.SetupRouter(cfg, userCtrl)

	redis.InitRedis()

	log.Printf("User service running on port %s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start user service: %v", err)
	}
}
