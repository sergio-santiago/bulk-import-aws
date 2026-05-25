output "uploads_bucket_name" {
  description = "S3 bucket where users PUT files via presigned URL."
  value       = aws_s3_bucket.uploads.id
}

output "uploads_bucket_arn" {
  description = "ARN of the uploads bucket."
  value       = aws_s3_bucket.uploads.arn
}

output "records_queue_url" {
  description = "URL of the main records queue."
  value       = aws_sqs_queue.records.url
}

output "records_queue_arn" {
  description = "ARN of the main records queue."
  value       = aws_sqs_queue.records.arn
}

output "records_dlq_url" {
  description = "URL of the dead-letter queue for failed records."
  value       = aws_sqs_queue.records_dlq.url
}

output "records_dlq_arn" {
  description = "ARN of the dead-letter queue for failed records."
  value       = aws_sqs_queue.records_dlq.arn
}
