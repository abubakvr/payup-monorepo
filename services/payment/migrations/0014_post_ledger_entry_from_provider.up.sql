-- Post a DEBIT ledger entry using provider (9PSB) balances when local balance is out of sync.
-- Sets app.allow_balance_update so the balance guard trigger allows the wallet UPDATE.
CREATE OR REPLACE FUNCTION post_ledger_entry_from_provider(
    p_transaction_id       UUID,
    p_wallet_id            UUID,
    p_amount               DECIMAL(18,2),
    p_available_after      DECIMAL(18,2),
    p_ledger_after         DECIMAL(18,2) DEFAULT NULL,
    p_narrative            VARCHAR(255) DEFAULT NULL
)
RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_balance_before    DECIMAL(18,2);
    v_balance_after     DECIMAL(18,2);
    v_ledger_after      DECIMAL(18,2);
    v_ledger_id         UUID;
BEGIN
    v_balance_after := p_available_after;
    v_balance_before := p_available_after + p_amount;
    v_ledger_after := COALESCE(p_ledger_after, p_available_after);

    INSERT INTO transaction_ledger
        (transaction_id, wallet_id, entry_type, amount,
         balance_before, balance_after, currency, narrative)
    VALUES
        (p_transaction_id, p_wallet_id, 'DEBIT', p_amount,
         v_balance_before, v_balance_after, 'NGN', p_narrative)
    RETURNING id INTO v_ledger_id;

    SET LOCAL app.allow_balance_update = 'true';

    UPDATE wallets
    SET    available_balance = v_balance_after,
           ledger_balance    = v_ledger_after,
           updated_at        = NOW()
    WHERE  id = p_wallet_id;

    RETURN v_ledger_id;
END;
$$;

COMMENT ON FUNCTION post_ledger_entry_from_provider IS
    'Posts a DEBIT using provider (9PSB) post-debit balances. Use when transfer succeeded at provider but local balance was stale. Sets app.allow_balance_update so guard allows the update.';
