# ADR 0001 — Event-driven serverless pipeline on AWS

Status: accepted, 2026-05-26.

## Context

`bulk-import-aws` ingests CSV files of products, validates each row and
persists the result. The brief frames the product as asynchronous and batch:
the user uploads a file, receives an `importId` immediately and reads the
report when processing is done. Volumes are sporadic (a handful of files
per day at most) and the project must stay cheap and reproducible.

Two more constraints shape the choice:

- It is a course project, not a long-lived service. Operational overhead has
  to be near zero.
- It runs in a heavily restricted lab account that does not allow creating
  IAM roles or users. Any stack that requires bespoke IAM principals would
  be a non-starter.

## Decision

Serverless event-driven pipeline on AWS:

- **S3** stores uploaded files (presigned PUT).
- **Lambda (`parser`)** is triggered by S3 `ObjectCreated`, parses the file
  and fans out one **SQS** message per row.
- **Lambda (`worker`)** consumes the queue, validates and persists each
  record into **DynamoDB** with per-message retry and a DLQ after three
  failed attempts.
- **Lambda (`api`)** sits behind **API Gateway HTTP API** with a Cognito
  **JWT authorizer** and serves the upload/listing endpoints.
- **CloudFront + S3** ship a single static HTML page.
- **CloudWatch alarm + SNS** notify when the DLQ is not empty.

All Lambdas share `studentLambdaExecutionRole`, the role pre-provisioned in
the lab account, and gain bucket-level S3 access via resource-based
policies.

## Alternatives considered

**Containers on ECS Fargate (or EKS) plus RDS.** Closer to a classical
backend: API and workers as containers behind an ALB, with PostgreSQL on
RDS. Discarded because it adds an ALB (~16 €/month), an always-on RDS
instance and the operational weight of an idle service that would mostly
sit unused. The asynchronous shape of the product does not benefit from
long-running processes.

**Step Functions orchestrating the import lifecycle.** A state machine per
import would express the flow nicely (parse → fan-out → reconcile) and
give you a visual log. Discarded because it is overkill for three steps
and ties the design to a service that is harder to reason about under
partial failures than plain SQS + Lambda.

**Hybrid: API on Lambda, workers on Fargate.** Useful when workers need
long execution, large memory or background concurrency unsupported by
Lambda. Discarded because each record processes in milliseconds; Lambda's
15-minute limit and 256 MB memory are not even close to being a
constraint.

## Consequences

**Positive.**

- Cost is essentially zero at rest; usage is dominated by free-tier
  allowances for Lambda, SQS, DynamoDB and S3.
- High availability comes for free from every managed service used.
- Failure modes are observable: validation failures surface in the
  per-import report, transient persistence failures end up in the DLQ
  with a CloudWatch alarm wired to SNS.
- Each layer is small and replaceable. The Go binaries are independent
  modules; the Terraform stack is split by concern (storage, queue,
  database, auth, compute, frontend, observability).

**Negative.**

- The lab constraints force a single IAM execution role shared across all
  Lambdas, breaking least-privilege-per-function. A real account would
  give each Lambda its own role with the minimum statements it needs.
- CI cannot deploy, only validate, because the lab does not allow creating
  IAM users or OIDC providers. Deploys are operated locally via SSO.
- Idempotency is best-effort: two uploads of the same file within
  milliseconds can race past the GSI check before either header is
  written. The realistic flow (upload, wait, retry on suspicion) is
  covered. A transactional claim on `file_hash` would close the window
  but is left for a follow-up.
- All Lambdas share `IMPORTS_TABLE` and `IMPORT_RECORDS_TABLE` access via
  the inline policies attached to the lab role; we cannot scope them to
  the project's tables without modifying a role we do not own.
