#!/usr/bin/env bash
# Uploads samples/with-errors.csv directly to the uploads bucket and polls
# the import header until done. Demonstrates that valid rows are processed
# while invalid ones are recorded as failed with their reason in the report.

set -euo pipefail

AWS_PROFILE="${AWS_PROFILE:-lab-sergio}"
export AWS_PROFILE

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
BUCKET="bulk-import-aws-uploads-${ACCOUNT_ID}"
TABLE_IMPORTS="bulk-import-aws-imports"
TABLE_RECORDS="bulk-import-aws-import-records"

USER_ID="demo-user-failure"
IMPORT_ID="proof-failure-$(date +%s)"
KEY="uploads/${USER_ID}/${IMPORT_ID}"

cd "$(dirname "$0")/.."

echo "==> uploading samples/with-errors.csv to s3://${BUCKET}/${KEY}"
aws s3 cp samples/with-errors.csv "s3://${BUCKET}/${KEY}" >/dev/null

echo "==> polling for completion"
for i in $(seq 1 30); do
    status=$(aws dynamodb get-item \
        --table-name "${TABLE_IMPORTS}" \
        --key "{\"import_id\":{\"S\":\"${IMPORT_ID}\"}}" \
        --query 'Item.status.S' --output text 2>/dev/null || echo "missing")
    printf "  [%02d] status=%s\n" "${i}" "${status}"
    if [ "${status}" = "done" ] || [ "${status}" = "failed" ]; then
        break
    fi
    sleep 2
done

echo
echo "==> import header"
aws dynamodb get-item \
    --table-name "${TABLE_IMPORTS}" \
    --key "{\"import_id\":{\"S\":\"${IMPORT_ID}\"}}" \
    --query 'Item.{status:status.S,total:total.N,succeeded:succeeded.N,failed:failed.N,finished_at:finished_at.S}'

echo
echo "==> failed records"
aws dynamodb query \
    --table-name "${TABLE_RECORDS}" \
    --key-condition-expression "import_id = :id" \
    --filter-expression "#s = :failed" \
    --expression-attribute-names '{"#s":"status"}' \
    --expression-attribute-values "{\":id\":{\"S\":\"${IMPORT_ID}\"},\":failed\":{\"S\":\"failed\"}}" \
    --query 'Items[].{record:record_id.S,row:row.N,error:error.S}'
