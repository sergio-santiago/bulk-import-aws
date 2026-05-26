# Proofs

Outputs of the mandatory checks from the project brief, captured against the
real AWS environment. Scripts under `scripts/` reproduce each run.

```
make package      # build lambdas (Docker)
make tf-apply     # deploy infra and code
AWS_PROFILE=lab-sergio ./scripts/prove-failure.sh
AWS_PROFILE=lab-sergio ./scripts/prove-idempotency.sh
AWS_PROFILE=lab-sergio ./scripts/prove-dlq.sh
```

## 1. Per-record validation failures end up in the report

Upload of `samples/with-errors.csv` (6 rows: 3 valid, 3 invalid).

```
==> uploading samples/with-errors.csv to s3://bulk-import-aws-uploads-883099621748/uploads/demo-user-failure/proof-failure-1779804520
==> polling for completion
  [01] status=None
  [02] status=done

==> import header
{
    "status": "done",
    "total": "6",
    "succeeded": "3",
    "failed": "3",
    "finished_at": "2026-05-26T14:08:43Z"
}

==> failed records
[
    { "record": "row-0002", "row": "2", "error": "sku is required" },
    { "record": "row-0004", "row": "4", "error": "name is required" },
    { "record": "row-0005", "row": "5", "error": "price must be greater than zero" }
]
```

What this shows:

- The three malformed rows are stored as `status=failed` records with the
  exact validation reason.
- The other three are processed normally (`succeeded=3`).
- The header transitions to `done` once `succeeded + failed == total`.

## 2. Re-uploading the same file does not duplicate

Two consecutive uploads of `samples/valid.csv` with different `import_id`s.

```
==> first upload as proof-idem-a-1779804533
==> second upload (identical content) as proof-idem-b-1779804533

==> first import (should hold the real records)
{
    "status": "done",
    "total": "5",
    "succeeded": "5",
    "duplicate_of": null
}

==> second import (should reference the first via duplicate_of, total=0)
{
    "status": "done",
    "total": "0",
    "succeeded": "0",
    "duplicate_of": "proof-idem-a-1779804533"
}

==> record count for first import:  5
==> record count for second import: 0
```

What this shows:

- The parser computes a SHA-256 of the uploaded file and queries the
  `file_hash-index` GSI before publishing any records.
- The second upload is stored as a header with `duplicate_of` pointing at the
  original import; zero records are dispatched to SQS so nothing is imported
  twice.

## 3. Persistence failures reach the DLQ and trigger the alarm

`prove-dlq.sh` temporarily reconfigures the worker's `IMPORT_RECORDS_TABLE`
env var to a non-existent table, uploads a unique CSV (so idempotency does
not short-circuit) and waits for SQS to redrive the messages after three
failed attempts. The script restores the original configuration on exit via
a `trap`.

```
==> point worker at a non-existent table to force persistence failures
==> uploading unique sample to trigger the pipeline
==> waiting up to ~4 min for retries + dlq redrive
  [01/24] dlq messages visible=0
  [02/24] dlq messages visible=0
  ...
  [20/24] dlq messages visible=0
  [21/24] dlq messages visible=3

==> sample dlq message (peeked, not deleted)
{"import_id":"proof-dlq-1779804992","record_id":"row-0002","row":2,"sku":"DLQ-1779804992-002","name":"DLQ Demo Two","price":20}
==> restoring worker env
```

CloudWatch alarm state after the DLQ filled up:

```
{
    "State": "ALARM",
    "Reason": "Threshold Crossed: 1 datapoint [3.0 (26/05/26 14:20:00)] was greater than the threshold (0.0).",
    "Updated": "2026-05-26T16:22:16.785000+02:00"
}
```

Alarm history confirms the SNS topic was notified:

```
{ "Type": "Action",       "Summary": "Successfully executed action arn:aws:sns:eu-west-1:883099621748:bulk-import-aws-alerts" }
{ "Type": "StateUpdate",  "Summary": "Alarm updated from OK to ALARM" }
```

What this shows:

- Records that cannot be persisted after three retries land on the DLQ with
  their full payload intact, so they can be inspected and (in a real
  system) replayed.
- The `bulk-import-aws-dlq-not-empty` alarm fires within one evaluation
  period (60 s) and triggers the SNS subscription, delivering an email to
  the configured address.
