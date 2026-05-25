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

output "imports_table_name" {
  description = "DynamoDB table holding import headers and their aggregated status."
  value       = aws_dynamodb_table.imports.name
}

output "imports_table_arn" {
  description = "ARN of the imports table."
  value       = aws_dynamodb_table.imports.arn
}

output "import_records_table_name" {
  description = "DynamoDB table holding individual records for each import."
  value       = aws_dynamodb_table.import_records.name
}

output "import_records_table_arn" {
  description = "ARN of the import records table."
  value       = aws_dynamodb_table.import_records.arn
}
