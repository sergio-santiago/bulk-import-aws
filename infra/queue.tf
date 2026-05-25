resource "aws_sqs_queue" "records_dlq" {
  name = "${local.name_prefix}-records-dlq"

  # Max retention so we keep failed messages around long enough to inspect
  # them in the AWS console or via logs.
  message_retention_seconds = 1209600 # 14 days

  sqs_managed_sse_enabled = true
}

resource "aws_sqs_queue" "records" {
  name = "${local.name_prefix}-records"

  visibility_timeout_seconds = 60
  message_retention_seconds  = 345600 # 4 days
  sqs_managed_sse_enabled    = true

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.records_dlq.arn
    maxReceiveCount     = 3
  })
}
