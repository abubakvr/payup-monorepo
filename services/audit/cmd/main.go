package main

import (
	"log"
	"net/http"
	"os"

	"github.com/abubakvr/payup-backend/services/audit/internal/config"
	"github.com/abubakvr/payup-backend/services/audit/internal/controller"
	"github.com/abubakvr/payup-backend/services/audit/internal/kafka"
	"github.com/abubakvr/payup-backend/services/audit/internal/repository"
	"github.com/abubakvr/payup-backend/services/audit/internal/router"
	"github.com/abubakvr/payup-backend/services/audit/internal/service"
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

	log.Printf("Audit service running on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
