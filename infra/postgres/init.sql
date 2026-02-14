-- USER SERVICE
CREATE DATABASE user_db;
CREATE USER user_service WITH PASSWORD 'user_password';
GRANT ALL PRIVILEGES ON DATABASE user_db TO user_service;
\c user_db
GRANT USAGE ON SCHEMA public TO user_service;
GRANT CREATE ON SCHEMA public TO user_service;

-- KYC SERVICE (reconnect to postgres to create next db)
\c postgres
CREATE DATABASE kyc_db;
CREATE USER kyc_service WITH PASSWORD 'kyc_password';
GRANT ALL PRIVILEGES ON DATABASE kyc_db TO kyc_service;
\c kyc_db
GRANT USAGE ON SCHEMA public TO kyc_service;
GRANT CREATE ON SCHEMA public TO kyc_service;
