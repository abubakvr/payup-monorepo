package auditgrpc

import (
	"context"
	"encoding/json"

	auditpb "github.com/abubakvr/payup-backend/proto/audit"
	"github.com/abubakvr/payup-backend/services/audit/internal/model"
	"github.com/abubakvr/payup-backend/services/audit/internal/service"
)

// AuditServer implements auditpb.AuditServiceServer for Admin service.
type AuditServer struct {
	auditpb.UnimplementedAuditServiceServer
	svc *service.AuditService
}

func NewAuditServer(svc *service.AuditService) *AuditServer {
	return &AuditServer{svc: svc}
}

func (s *AuditServer) GetUserAudits(ctx context.Context, req *auditpb.UserAuditRequest) (*auditpb.AuditResponse, error) {
	if req == nil || req.UserId == "" {
		return &auditpb.AuditResponse{Logs: nil, Total: 0}, nil
	}
	limit, offset := int(req.Limit), int(req.Offset)
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	logs, total, err := s.svc.GetByUserPaginated(req.UserId, limit, offset)
	if err != nil {
		return nil, err
	}
	return &auditpb.AuditResponse{Logs: mapAuditEvents(logs), Total: total}, nil
}

func (s *AuditServer) ListAllAudits(ctx context.Context, req *auditpb.ListAllAuditsRequest) (*auditpb.AuditResponse, error) {
	limit, offset := int(req.Limit), int(req.Offset)
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	logs, total, err := s.svc.GetAllPaginated(limit, offset)
	if err != nil {
		return nil, err
	}
	return &auditpb.AuditResponse{Logs: mapAuditEvents(logs), Total: total}, nil
}

func mapAuditEvents(events []model.AuditEvent) []*auditpb.AuditLog {
	out := make([]*auditpb.AuditLog, 0, len(events))
	for _, ev := range events {
		userID := ""
		if ev.UserID != nil {
			userID = *ev.UserID
		}
		entityID := ""
		if ev.EntityID != nil {
			entityID = *ev.EntityID
		}
		metaJSON := ""
		if ev.Metadata != nil {
			b, _ := json.Marshal(ev.Metadata)
			metaJSON = string(b)
		}
		out = append(out, &auditpb.AuditLog{
			Service:      ev.Service,
			UserId:       userID,
			Action:       ev.Action,
			Entity:       ev.Entity,
			EntityId:     entityID,
			MetadataJson: metaJSON,
			CreatedAt:    ev.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return out
}
