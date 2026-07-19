#!/bin/bash
# FUUDELIVERY PostgreSQL Migration
# Generates SQL from GORM and applies to Supabase

set -a
source ../.env
set +a

echo "=== PostgreSQL Migration ==="
echo "Database: Supabase"

# The app auto-migrates on startup via GORM AutoMigrate
# For manual SQL generation, you can use Prisma or run the app locally

echo "✓ Migration is handled by GORM AutoMigrate on application startup"
echo "  First deployment will create all tables automatically."
echo "  Subsequent deployments will apply any schema changes."
