-- Creates payment_db and payment_service so payment-migrate can run.
-- Run via payment-db-ensure service. Safe to run multiple times with ON_ERROR_STOP=0.

\c postgres
CREATE DATABASE payment_db;
CREATE USER payment_service WITH PASSWORD :'payment_password';
GRANT ALL PRIVILEGES ON DATABASE payment_db TO payment_service;
\c payment_db
GRANT USAGE ON SCHEMA public TO payment_service;
GRANT CREATE ON SCHEMA public TO payment_service;
