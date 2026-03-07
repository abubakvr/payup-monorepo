DROP INDEX IF EXISTS idx_webhook_received_at;
DROP INDEX IF EXISTS idx_webhook_processing_status_received;
DROP INDEX IF EXISTS idx_webhook_event_type_status;
DROP INDEX IF EXISTS idx_webhook_account_hash;
DROP INDEX IF EXISTS idx_webhook_transaction_ref;
DROP INDEX IF EXISTS ux_webhook_provider_ref;
DROP TABLE IF EXISTS webhook_events;
