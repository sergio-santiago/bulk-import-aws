output "state_bucket" {
  description = "S3 bucket name for the remote Terraform state."
  value       = aws_s3_bucket.tfstate.id
}

output "region" {
  description = "AWS region where the bootstrap resources live."
  value       = var.region
}
