-- Creates admin_db and admin_service so admin-migrate can run.
-- Run automatically by the admin-db-ensure service on every "docker compose up".
-- Safe to run multiple times: ON_ERROR_STOP=0 is used so "already exists" is ignored.
--
-- Manual run: docker exec -i payup-postgres2 psql -U postgres < infra/postgres/create-admin-role.sql

\c postgres
CREATE DATABASE admin_db;
CREATE USER admin_service WITH PASSWORD 'admin_password';
GRANT ALL PRIVILEGES ON DATABASE admin_db TO admin_service;
\c admin_db
GRANT USAGE ON SCHEMA public TO admin_service;
GRANT CREATE ON SCHEMA public TO admin_service;
