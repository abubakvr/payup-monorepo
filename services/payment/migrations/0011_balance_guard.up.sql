CREATE OR REPLACE FUNCTION guard_balance_update()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF (NEW.ledger_balance    IS DISTINCT FROM OLD.ledger_balance OR
        NEW.available_balance IS DISTINCT FROM OLD.available_balance)
    AND current_setting('app.allow_balance_update', true) IS DISTINCT FROM 'true'
    THEN
        RAISE EXCEPTION
            'Direct balance update is forbidden. Use post_ledger_entry() instead. '
            'wallet_id=%, attempted ledger=%, available=%',
            NEW.id, NEW.ledger_balance, NEW.available_balance;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_wallets_balance_guard
    BEFORE UPDATE ON wallets
    FOR EACH ROW EXECUTE FUNCTION guard_balance_update();

COMMENT ON FUNCTION guard_balance_update IS
    'Blocks direct UPDATE of ledger_balance/available_balance unless '
    'app.allow_balance_update=true is set in the current transaction. '
    'Only post_ledger_entry() should set this flag.';
