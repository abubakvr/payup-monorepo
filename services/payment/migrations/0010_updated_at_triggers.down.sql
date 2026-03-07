DROP TRIGGER IF EXISTS trg_wallet_upgrade_updated_at ON wallet_upgrade_requests;
DROP TRIGGER IF EXISTS trg_transactions_updated_at ON transactions;
DROP TRIGGER IF EXISTS trg_wallets_updated_at ON wallets;
DROP TRIGGER IF EXISTS trg_auth_tokens_updated_at ON auth_tokens;
DROP FUNCTION IF EXISTS set_updated_at();
