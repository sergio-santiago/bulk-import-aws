#!/usr/bin/env bash
# Uploads samples/valid.csv twice with different import_ids. Demonstrates
# that the second upload is detected as a duplicate (same file_hash) and
# is stored with status=done, duplicate_of=<first>, total=0 — no records
# are re-imported.

set -euo pipefail

AWS_PROFILE="${AWS_PROFILE:-lab-sergio}"
export AWS_PROFILE

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
BUCKET="bulk-import-aws-uploads-${ACCOUNT_ID}"
TABLE_IMPORTS="bulk-import-aws-imports"
TABLE_RECORDS="bulk-import-aws-import-records"

USER_ID="demo-user-idempotency"
SUFFIX="$(date +%s)"
IMPORT_A="proof-idem-a-${SUFFIX}"
IMPORT_B="proof-idem-b-${SUFFIX}"

cd "$(dirname "$0")/.."

echo "==> first upload as ${IMPORT_A}"
aws s3 cp samples/valid.csv "s3://${BUCKET}/uploads/${USER_ID}/${IMPORT_A}" >/dev/null
echo "  waiting 10s for parser to write the original header..."
sleep 10

echo "==> second upload (identical content) as ${IMPORT_B}"
aws s3 cp samples/valid.csv "s3://${BUCKET}/uploads/${USER_ID}/${IMPORT_B}" >/dev/null
echo "  waiting 10s for parser to detect the duplicate..."
sleep 10

echo
echo "==> first import (should hold the real records)"
aws dynamodb get-item \
    --table-name "${TABLE_IMPORTS}" \
    --key "{\"import_id\":{\"S\":\"${IMPORT_A}\"}}" \
    --query 'Item.{status:status.S,total:total.N,succeeded:succeeded.N,duplicate_of:duplicate_of.S}'

echo
echo "==> second import (should reference the first via duplicate_of, total=0)"
aws dynamodb get-item \
    --table-name "${TABLE_IMPORTS}" \
    --key "{\"import_id\":{\"S\":\"${IMPORT_B}\"}}" \
    --query 'Item.{status:status.S,total:total.N,succeeded:succeeded.N,duplicate_of:duplicate_of.S}'

count_records() {
    aws dynamodb query \
        --table-name "${TABLE_RECORDS}" \
        --key-condition-expression "import_id = :id" \
        --expression-attribute-values "{\":id\":{\"S\":\"$1\"}}" \
        --select COUNT --query 'Count'
}

echo
echo "==> record count for first import:  $(count_records "${IMPORT_A}")"
echo "==> record count for second import: $(count_records "${IMPORT_B}")"
