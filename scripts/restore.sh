#!/bin/bash
# Restore a FabAID backup into a development environment.
# Usage: ./scripts/restore.sh <backup-tarball>
#
# Environment variables:
#   DATABASE_URL  - PostgreSQL connection string
#   S3_ENDPOINT   - MinIO/S3 endpoint
#   S3_BUCKET     - Bucket name
#   S3_ACCESS_KEY - S3 access key
#   S3_SECRET_KEY - S3 secret key

set -euo pipefail

BACKUP_FILE="${1:?Usage: restore.sh <backup.tar.gz>}"
WORKDIR=$(mktemp -d)

echo "==> Extracting backup to $WORKDIR..."
tar -xzf "$BACKUP_FILE" -C "$WORKDIR"

# Restore database
if [ -f "$WORKDIR/database/fabaid.sql" ]; then
    echo "==> Restoring database..."
    psql "$DATABASE_URL" < "$WORKDIR/database/fabaid.sql"
    echo "    Database restored."
else
    echo "    WARNING: No database dump found in backup."
fi

# Restore documents to S3
if [ -d "$WORKDIR/documents" ]; then
    echo "==> Restoring documents to S3..."
    # Configure mc (MinIO client)
    mc alias set fabaid "$S3_ENDPOINT" "$S3_ACCESS_KEY" "$S3_SECRET_KEY" 2>/dev/null || true
    mc mb --ignore-existing "fabaid/$S3_BUCKET" 2>/dev/null || true

    find "$WORKDIR/documents" -type f | while read -r filepath; do
        # Strip the workdir/documents/ prefix to get the S3 key
        s3key="${filepath#$WORKDIR/documents/}"
        echo "    Uploading: $s3key"
        mc cp "$filepath" "fabaid/$S3_BUCKET/$s3key"
    done
    echo "    Documents restored."
else
    echo "    WARNING: No documents directory found in backup."
fi

echo "==> Cleanup..."
rm -rf "$WORKDIR"
echo "==> Restore complete!"
