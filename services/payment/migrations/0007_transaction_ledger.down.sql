DROP RULE IF EXISTS no_delete_ledger ON transaction_ledger;
DROP RULE IF EXISTS no_update_ledger ON transaction_ledger;
DROP INDEX IF EXISTS idx_ledger_txn_entry_type;
DROP INDEX IF EXISTS idx_ledger_entry_type_created_at;
DROP INDEX IF EXISTS idx_ledger_created_at;
DROP INDEX IF EXISTS idx_ledger_wallet_date;
DROP INDEX IF EXISTS idx_ledger_transaction_id;
DROP TABLE IF EXISTS transaction_ledger;
