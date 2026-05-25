output "state_bucket" {
  description = "S3 bucket name for the remote Terraform state."
  value       = aws_s3_bucket.tfstate.id
}

output "lock_table" {
  description = "DynamoDB table name for state locking."
  value       = aws_dynamodb_table.tfstate_lock.id
}

output "region" {
  description = "AWS region where the bootstrap resources live."
  value       = var.region
}
