#!/bin/bash
# FUUDELIVERY Database Setup Script
# Run this once to set up all databases

echo "=== FUUDELIVERY Database Setup ==="

# Load env vars
set -a
source ../.env
set +a

# 1. PostgreSQL (Supabase) - Tables are auto-migrated by GORM
echo "✓ PostgreSQL tables will be auto-created by the app on first run"

# 2. MongoDB (Atlas) - Collections are created on first use
echo "✓ MongoDB collections will be auto-created on first use"

# 3. Create MongoDB indexes for better performance
echo "Creating MongoDB indexes..."
cat << 'EOF' | mongosh "$MONGODB_ATLAS_URI" --quiet
    db.orders.createIndex({ "establishmentid": 1 });
    db.orders.createIndex({ "user.phone": 1 });
    db.orders.createIndex({ "status": 1 });
    db.orders.createIndex({ "lastModified": -1 });
    
    db.solicitations.createIndex({ "orderid": 1 }, { unique: true });
    db.solicitations.createIndex({ "deliveryman.id": 1 });
    db.solicitations.createIndex({ "status": 1 });
    
    db.payments.createIndex({ "order_id": 1 });
    db.payments.createIndex({ "customer_id": 1 });
    db.payments.createIndex({ "status": 1 });
    
    db.wallets.createIndex({ "user_id": 1, "user_type": 1 }, { unique: true });
    
    db.chat_messages.createIndex({ "order_id": 1 });
    db.chat_messages.createIndex({ "created_at": 1 });
    
    db.push_tokens.createIndex({ "user_id": 1, "user_type": 1 }, { unique: true });
    
    print("✓ Indexes created successfully");
EOF
