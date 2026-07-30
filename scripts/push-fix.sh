#!/bin/bash
cd /home/daytona/codebase

# Move sensitive files out of the way
mkdir -p /tmp/fuu-env-backup
find . -name "_simple*" -path "*/orders_api/*" -exec mv {} /tmp/fuu-env-backup/ \; 2>/dev/null
find . -name ".env*" -path "*/orders_api/*" ! -name ".gitignore" -exec mv {} /tmp/fuu-env-backup/ \; 2>/dev/null

echo "Files moved out of way"

# Pull with rebase
git pull --rebase origin master 2>&1
echo "---REBASE DONE---"

# Now remove from tracking again (remote re-added them)
find . -name "_simple*" -path "*/orders_api/*" -exec git rm --cached -f {} \; 2>/dev/null
find . -name ".env*" -path "*/orders_api/*" ! -name ".gitignore" -exec git rm --cached -f {} \; 2>/dev/null
echo "---CACHED REMOVAL DONE---"

# Commit the removal
git add -A 2>/dev/null
git commit -m 'chore: remove streadway/amqp from all go.mod, untrack sensitive files' 2>&1 || echo "Nothing to commit"

# Push
git push origin master 2>&1
echo "---PUSH DONE---"
