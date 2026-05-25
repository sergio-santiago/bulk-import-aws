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

output "user_pool_id" {
  description = "Cognito user pool ID."
  value       = aws_cognito_user_pool.users.id
}

output "user_pool_arn" {
  description = "Cognito user pool ARN. Used by the API Gateway JWT authorizer."
  value       = aws_cognito_user_pool.users.arn
}

output "user_pool_client_id" {
  description = "Cognito app client ID used by the web frontend."
  value       = aws_cognito_user_pool_client.web.id
}

output "cognito_hosted_ui_domain" {
  description = "Base URL of the Cognito hosted UI."
  value       = "https://${aws_cognito_user_pool_domain.users.domain}.auth.${var.region}.amazoncognito.com"
}

output "lambda_function_names" {
  description = "Map of lambda name to deployed function name."
  value       = { for k, v in aws_lambda_function.this : k => v.function_name }
}

output "lambda_function_arns" {
  description = "Map of lambda name to deployed function ARN."
  value       = { for k, v in aws_lambda_function.this : k => v.arn }
}

output "api_endpoint" {
  description = "Base invoke URL of the HTTP API."
  value       = aws_apigatewayv2_stage.default.invoke_url
}

output "web_bucket_name" {
  description = "S3 bucket serving the static frontend."
  value       = aws_s3_bucket.web.id
}

output "cloudfront_domain" {
  description = "Public HTTPS URL of the CloudFront distribution."
  value       = "https://${aws_cloudfront_distribution.web.domain_name}"
}

output "cloudfront_distribution_id" {
  description = "Distribution ID, used for cache invalidations."
  value       = aws_cloudfront_distribution.web.id
}
