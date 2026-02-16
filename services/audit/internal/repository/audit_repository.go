package repository

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/abubakvr/payup-backend/services/audit/internal/model"
)

type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Insert(event model.AuditEvent) error {
	metadata, _ := json.Marshal(event.Metadata)

	query := `
		INSERT INTO audit_logs (service, user_id, action, entity, entity_id, metadata, correlation_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Exec(query, event.Service, event.UserID, event.Action, event.Entity, event.EntityID, metadata, event.CorrelationID, event.Timestamp)
	return err
}

func (r *AuditRepository) GetLogs(filter model.AuditFilter) ([]model.AuditEvent, error) {
	query := `
		SELECT id, service, user_id, action, entity, entity_id, metadata, correlation_id, created_at
		FROM audit_logs
		WHERE 1=1
	`

	args := []interface{}{}

	if filter.Service != "" {
		query += " AND service = $1"
		args = append(args, filter.Service)
	}
	if filter.EntityID != nil {
		query += " AND entity_id = $1"
		args = append(args, *filter.EntityID)
	}
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := []model.AuditEvent{}
	for rows.Next() {
		var log model.AuditEvent
		var metadata json.RawMessage
		err := rows.Scan(&log.EntityID, &log.Service, &log.UserID, &log.Action, &log.Entity, &log.EntityID, &metadata, &log.CorrelationID, &log.Timestamp)
		if err != nil {
			return nil, err
		}
		json.Unmarshal(metadata, &log.Metadata)
		logs = append(logs, log)
	}
	return logs, nil
}

func (r *AuditRepository) GetByUser(userId string) ([]model.AuditEvent, error) {
	query := `SELECT id, service, action, entity, metadata, correlation_id, created_at FROM audit_logs WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.Query(query, userId)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	var logs []model.AuditEvent

	for rows.Next() {
		var id int64
		var service, action, entity, correlationId string
		var metadataRaw []byte
		var createdAt time.Time

		rows.Scan(&id, &service, &action, &entity, &metadataRaw, &correlationId, &createdAt)

		var metadata map[string]interface{}
		json.Unmarshal(metadataRaw, &metadata)

		logs = append(logs, model.AuditEvent{
			Service:       service,
			Action:        action,
			Entity:        entity,
			Metadata:      metadata,
			CorrelationID: &correlationId,
			Timestamp:     createdAt,
		})
	}

	return logs, nil
}
