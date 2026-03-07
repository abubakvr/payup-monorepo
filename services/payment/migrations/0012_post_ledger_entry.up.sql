CREATE OR REPLACE FUNCTION post_ledger_entry(
    p_transaction_id    UUID,
    p_wallet_id         UUID,
    p_entry_type        ledger_entry_type,
    p_amount            DECIMAL(18,2),
    p_currency          VARCHAR(3)  DEFAULT 'NGN',
    p_narrative         VARCHAR(255) DEFAULT NULL
)
RETURNS UUID
LANGUAGE plpgsql
AS $$
DECLARE
    v_balance_before    DECIMAL(18,2);
    v_balance_after     DECIMAL(18,2);
    v_ledger_id         UUID;
BEGIN
    SELECT available_balance
    INTO   v_balance_before
    FROM   wallets
    WHERE  id = p_wallet_id
    FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'Wallet not found: %', p_wallet_id;
    END IF;

    IF p_entry_type = 'DEBIT' THEN
        v_balance_after := v_balance_before - p_amount;
    ELSE
        v_balance_after := v_balance_before + p_amount;
    END IF;

    IF v_balance_after < 0 THEN
        RAISE EXCEPTION
            'Insufficient balance. wallet_id=%, balance=%, debit_amount=%',
            p_wallet_id, v_balance_before, p_amount;
    END IF;

    INSERT INTO transaction_ledger
        (transaction_id, wallet_id, entry_type, amount,
         balance_before, balance_after, currency, narrative)
    VALUES
        (p_transaction_id, p_wallet_id, p_entry_type, p_amount,
         v_balance_before, v_balance_after, p_currency, p_narrative)
    RETURNING id INTO v_ledger_id;

    SET LOCAL app.allow_balance_update = 'true';

    UPDATE wallets
    SET    available_balance = v_balance_after,
           ledger_balance    = CASE
                                   WHEN p_entry_type = 'DEBIT'
                                   THEN ledger_balance - p_amount
                                   ELSE ledger_balance + p_amount
                               END
    WHERE  id = p_wallet_id;

    RETURN v_ledger_id;
END;
$$;

COMMENT ON FUNCTION post_ledger_entry IS
    'The ONLY authorised way to post a ledger entry and update wallet balances. '
    'Acquires row lock on wallets, posts to transaction_ledger, updates both '
    'ledger_balance and available_balance atomically. '
    'Raises if balance would go negative.';
