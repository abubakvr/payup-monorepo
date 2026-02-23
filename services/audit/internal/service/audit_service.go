package service

import (
	"github.com/abubakvr/payup-backend/services/audit/internal/model"
	"github.com/abubakvr/payup-backend/services/audit/internal/repository"
)

type AuditService struct {
	repo *repository.AuditRepository
}

func NewAuditService(repo *repository.AuditRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) ProcessAuditEvent(event model.AuditEvent) error {
	return s.repo.Insert(event)
}

func (s *AuditService) GetLogs(filter model.AuditFilter) ([]model.AuditEvent, error) {
	return s.repo.GetLogs(filter)
}

func (s *AuditService) GetAll() ([]model.AuditEvent, error) {
	return s.repo.GetAll()
}

func (s *AuditService) GetByUser(userId string) ([]model.AuditEvent, error) {
	return s.repo.GetByUser(userId)
}
