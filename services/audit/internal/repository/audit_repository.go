package repository

import (
	"database/sql"
	"encoding/json"

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

func (r *AuditRepository) GetAll() ([]model.AuditEvent, error) {
	logs, _, err := r.GetAllPaginated(0, 0)
	return logs, err
}

func (r *AuditRepository) GetAllPaginated(limit, offset int) ([]model.AuditEvent, int64, error) {
	countQuery := `SELECT COUNT(*) FROM audit_logs`
	var total int64
	if err := r.db.QueryRow(countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT service, user_id, action, entity, entity_id, metadata, correlation_id, created_at FROM audit_logs ORDER BY created_at DESC`
	if limit > 0 {
		query += ` LIMIT $1 OFFSET $2`
	}
	args := []interface{}{}
	if limit > 0 {
		args = append(args, limit, offset)
	}
	var rows *sql.Rows
	var err error
	if len(args) > 0 {
		rows, err = r.db.Query(query, args...)
	} else {
		rows, err = r.db.Query(query)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	logs, err := scanAuditRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (r *AuditRepository) GetByUser(userId string) ([]model.AuditEvent, error) {
	logs, _, err := r.GetByUserPaginated(userId, 0, 0)
	return logs, err
}

func (r *AuditRepository) GetByUserPaginated(userId string, limit, offset int) ([]model.AuditEvent, int64, error) {
	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE user_id = $1`
	var total int64
	if err := r.db.QueryRow(countQuery, userId).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT service, user_id, action, entity, entity_id, metadata, correlation_id, created_at FROM audit_logs WHERE user_id = $1 ORDER BY created_at DESC`
	args := []interface{}{userId}
	if limit > 0 {
		query += ` LIMIT $2 OFFSET $3`
		args = append(args, limit, offset)
	}
	var rows *sql.Rows
	var err error
	if len(args) > 1 {
		rows, err = r.db.Query(query, args...)
	} else {
		rows, err = r.db.Query(query, args...)
	}
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	logs, err := scanAuditRows(rows)
	if err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func scanAuditRows(rows *sql.Rows) ([]model.AuditEvent, error) {
	var logs []model.AuditEvent
	for rows.Next() {
		var e model.AuditEvent
		var userID, entityID, correlationID sql.NullString
		var metadataRaw []byte
		if err := rows.Scan(&e.Service, &userID, &e.Action, &e.Entity, &entityID, &metadataRaw, &correlationID, &e.Timestamp); err != nil {
			return nil, err
		}
		if userID.Valid {
			e.UserID = &userID.String
		}
		if entityID.Valid {
			e.EntityID = &entityID.String
		}
		if correlationID.Valid {
			e.CorrelationID = &correlationID.String
		}
		_ = json.Unmarshal(metadataRaw, &e.Metadata)
		logs = append(logs, e)
	}
	return logs, nil
}
