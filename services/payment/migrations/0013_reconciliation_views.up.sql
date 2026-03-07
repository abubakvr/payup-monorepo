-- Daily double-entry balance check: rows only when DEBIT total != CREDIT total
CREATE OR REPLACE VIEW v_daily_ledger_check AS
SELECT
    (created_at::DATE)                                              AS day,
    SUM(CASE WHEN entry_type = 'DEBIT'  THEN amount ELSE 0 END)    AS total_debits,
    SUM(CASE WHEN entry_type = 'CREDIT' THEN amount ELSE 0 END)    AS total_credits,
    SUM(CASE WHEN entry_type = 'DEBIT'  THEN amount ELSE 0 END) -
    SUM(CASE WHEN entry_type = 'CREDIT' THEN amount ELSE 0 END)    AS imbalance
FROM   transaction_ledger
GROUP  BY (created_at::DATE)
HAVING SUM(CASE WHEN entry_type = 'DEBIT'  THEN amount ELSE 0 END) <>
       SUM(CASE WHEN entry_type = 'CREDIT' THEN amount ELSE 0 END);

COMMENT ON VIEW v_daily_ledger_check IS
    'Returns one row per day where DEBIT total != CREDIT total. Zero rows = healthy.';

-- SUCCESS transactions with mismatched or missing ledger entries
CREATE OR REPLACE VIEW v_ledger_gaps AS
SELECT
    t.id,
    t.transaction_ref,
    t.type,
    t.status,
    t.amount,
    t.is_reconciled,
    t.created_at,
    COUNT(CASE WHEN l.entry_type = 'DEBIT'  THEN 1 END)    AS debit_count,
    COUNT(CASE WHEN l.entry_type = 'CREDIT' THEN 1 END)    AS credit_count
FROM   transactions t
LEFT   JOIN transaction_ledger l ON l.transaction_id = t.id
WHERE  t.status = 'SUCCESS'
GROUP  BY t.id, t.transaction_ref, t.type, t.status,
          t.amount, t.is_reconciled, t.created_at
HAVING
    (t.type != 'OUTBOUND_TRANSFER' AND
     COUNT(CASE WHEN l.entry_type = 'DEBIT'  THEN 1 END) <>
     COUNT(CASE WHEN l.entry_type = 'CREDIT' THEN 1 END))
    OR
    COUNT(l.id) = 0;

COMMENT ON VIEW v_ledger_gaps IS
    'SUCCESS transactions with mismatched or missing ledger entries. Requires investigation.';

-- Open transfers awaiting webhook (debited but no CREDIT yet)
CREATE OR REPLACE VIEW v_pending_webhook_transfers AS
SELECT
    t.id,
    t.transaction_ref,
    t.provider_ref,
    t.amount,
    t.status,
    t.requery_count,
    t.created_at,
    EXTRACT(EPOCH FROM (NOW() - t.created_at)) / 60  AS age_minutes
FROM   transactions t
WHERE  t.type   = 'OUTBOUND_TRANSFER'
  AND  t.status IN ('SUCCESS', 'PENDING', 'REQUIRES_REQUERY')
  AND  NOT EXISTS (
           SELECT 1 FROM transaction_ledger l
           WHERE  l.transaction_id = t.id
             AND  l.entry_type = 'CREDIT'
       );

COMMENT ON VIEW v_pending_webhook_transfers IS
    'Outbound transfers debited but CREDIT (webhook) not yet arrived. Age > 60 min = investigate.';

-- Stale PENDING transactions for requery worker
CREATE OR REPLACE VIEW v_stale_pending AS
SELECT
    id,
    transaction_ref,
    provider_ref,
    wallet_id,
    type,
    amount,
    requery_count,
    last_requeried_at,
    created_at,
    EXTRACT(EPOCH FROM (NOW() - created_at)) / 60   AS age_minutes
FROM   transactions
WHERE  status IN ('PENDING', 'REQUIRES_REQUERY')
  AND  created_at < NOW() - INTERVAL '30 minutes'
ORDER  BY created_at ASC;

COMMENT ON VIEW v_stale_pending IS
    'Transactions stuck in PENDING or REQUIRES_REQUERY for >30 minutes. For requery worker.';

-- Daily finance summary
CREATE OR REPLACE VIEW v_daily_summary AS
SELECT
    (created_at::DATE)  AS day,
    type,
    status,
    direction,
    COUNT(*)            AS txn_count,
    SUM(amount)         AS total_amount,
    SUM(fee_amount)     AS total_fees
FROM   transactions
GROUP  BY (created_at::DATE), type, status, direction
ORDER  BY day DESC, type, status;

COMMENT ON VIEW v_daily_summary IS
    'Daily transaction totals by type, status, and direction. For finance reporting.';
