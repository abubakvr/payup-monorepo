CREATE TABLE transaction_ledger (
    id              UUID            NOT NULL DEFAULT gen_random_uuid(),
    transaction_id  UUID            NOT NULL,
    wallet_id       UUID            NOT NULL,
    entry_type      ledger_entry_type NOT NULL,
    amount          DECIMAL(18,2)   NOT NULL,
    balance_before  DECIMAL(18,2)   NOT NULL,
    balance_after   DECIMAL(18,2)   NOT NULL,
    currency        VARCHAR(3)      NOT NULL DEFAULT 'NGN',
    narrative       VARCHAR(255),
    created_at      TIMESTAMPTZ     NOT NULL DEFAULT NOW(),

    CONSTRAINT transaction_ledger_pkey          PRIMARY KEY (id),
    CONSTRAINT transaction_ledger_amount_pos    CHECK (amount > 0),
    CONSTRAINT transaction_ledger_txn_fk        FOREIGN KEY (transaction_id)
                                                    REFERENCES transactions (id)
                                                    ON DELETE RESTRICT,
    CONSTRAINT transaction_ledger_wallet_fk     FOREIGN KEY (wallet_id)
                                                    REFERENCES wallets (id)
                                                    ON DELETE RESTRICT
);

COMMENT ON TABLE transaction_ledger IS 'Append-only double-entry ledger. No UPDATE, no DELETE. Correcting entries only.';
COMMENT ON COLUMN transaction_ledger.entry_type IS 'DEBIT = money out of wallet. CREDIT = money into wallet.';
COMMENT ON COLUMN transaction_ledger.balance_before IS 'wallets.available_balance snapshot immediately before this entry';
COMMENT ON COLUMN transaction_ledger.balance_after IS 'wallets.available_balance snapshot immediately after this entry';

CREATE INDEX idx_ledger_transaction_id ON transaction_ledger (transaction_id);
CREATE INDEX idx_ledger_wallet_date ON transaction_ledger (wallet_id, created_at DESC);
CREATE INDEX idx_ledger_created_at ON transaction_ledger (created_at DESC);
CREATE INDEX idx_ledger_entry_type_created_at ON transaction_ledger (entry_type, created_at);
CREATE INDEX idx_ledger_txn_entry_type ON transaction_ledger (transaction_id, entry_type);

CREATE RULE no_update_ledger AS ON UPDATE TO transaction_ledger DO INSTEAD NOTHING;
CREATE RULE no_delete_ledger AS ON DELETE TO transaction_ledger DO INSTEAD NOTHING;
