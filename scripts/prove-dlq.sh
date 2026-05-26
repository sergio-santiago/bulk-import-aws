#!/usr/bin/env bash
# Provokes a persistence failure in the worker by pointing its
# IMPORT_RECORDS_TABLE env var at a non-existent table. With
# maxReceiveCount=3 and visibility_timeout=60s, messages land in the DLQ
# after ~3-4 minutes. The trap on EXIT always restores the original
# configuration so the system is left healthy.

set -euo pipefail

AWS_PROFILE="${AWS_PROFILE:-lab-sergio}"
export AWS_PROFILE

WORKER="bulk-import-aws-worker"

ACCOUNT_ID="$(aws sts get-caller-identity --query Account --output text)"
BUCKET="bulk-import-aws-uploads-${ACCOUNT_ID}"
DLQ_URL="https://sqs.eu-west-1.amazonaws.com/${ACCOUNT_ID}/bulk-import-aws-records-dlq"

USER_ID="demo-user-dlq"
SUFFIX="$(date +%s)"
IMPORT_ID="proof-dlq-${SUFFIX}"

cd "$(dirname "$0")/.."

# Unique CSV per run; otherwise the parser would short-circuit via the
# idempotency check and the worker would never be invoked.
TMPFILE="$(mktemp -t bulk-dlq-XXXXXX.csv)"
cat > "${TMPFILE}" <<EOF
sku,name,price
DLQ-${SUFFIX}-001,DLQ Demo One,10.00
DLQ-${SUFFIX}-002,DLQ Demo Two,20.00
DLQ-${SUFFIX}-003,DLQ Demo Three,30.00
EOF

cleanup() {
    rm -f "${TMPFILE}"
    echo "==> restoring worker env"
    aws lambda update-function-configuration \
        --function-name "${WORKER}" \
        --environment "Variables={IMPORT_RECORDS_TABLE=bulk-import-aws-import-records,IMPORTS_TABLE=bulk-import-aws-imports}" \
        >/dev/null
}
trap cleanup EXIT

echo "==> point worker at a non-existent table to force persistence failures"
aws lambda update-function-configuration \
    --function-name "${WORKER}" \
    --environment "Variables={IMPORT_RECORDS_TABLE=does-not-exist,IMPORTS_TABLE=bulk-import-aws-imports}" \
    >/dev/null

echo "  waiting 5s for the env update to propagate..."
sleep 5

echo "==> uploading unique sample to trigger the pipeline"
aws s3 cp "${TMPFILE}" "s3://${BUCKET}/uploads/${USER_ID}/${IMPORT_ID}" >/dev/null

echo "==> waiting up to ~4 min for retries + dlq redrive"
for i in $(seq 1 24); do
    visible=$(aws sqs get-queue-attributes \
        --queue-url "${DLQ_URL}" \
        --attribute-names ApproximateNumberOfMessages \
        --query 'Attributes.ApproximateNumberOfMessages' --output text)
    printf "  [%02d/24] dlq messages visible=%s\n" "${i}" "${visible}"
    if [ "${visible}" -gt 0 ]; then
        break
    fi
    sleep 10
done

echo
echo "==> dlq attributes"
aws sqs get-queue-attributes \
    --queue-url "${DLQ_URL}" \
    --attribute-names ApproximateNumberOfMessages

echo
echo "==> sample dlq message (peeked, not deleted)"
aws sqs receive-message \
    --queue-url "${DLQ_URL}" \
    --max-number-of-messages 1 \
    --visibility-timeout 0 \
    --query 'Messages[0].Body' --output text
