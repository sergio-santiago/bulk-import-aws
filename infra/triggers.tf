# Allow S3 to invoke the parser Lambda for object-created events on the
# uploads bucket. source_arn pins this to our bucket only.
resource "aws_lambda_permission" "s3_invoke_parser" {
  statement_id  = "AllowS3InvokeParser"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.this["parser"].function_name
  principal     = "s3.amazonaws.com"
  source_arn    = aws_s3_bucket.uploads.arn
}

resource "aws_s3_bucket_notification" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  lambda_function {
    lambda_function_arn = aws_lambda_function.this["parser"].arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "uploads/"
  }

  depends_on = [aws_lambda_permission.s3_invoke_parser]
}

# Wire the SQS records queue to the worker Lambda. ReportBatchItemFailures
# lets the worker signal per-message failures so only the failed ones get
# retried (and eventually moved to the DLQ).
resource "aws_lambda_event_source_mapping" "worker" {
  event_source_arn        = aws_sqs_queue.records.arn
  function_name           = aws_lambda_function.this["worker"].arn
  batch_size              = 10
  function_response_types = ["ReportBatchItemFailures"]
}
