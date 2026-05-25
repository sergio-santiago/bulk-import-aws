output "uploads_bucket_name" {
  description = "S3 bucket where users PUT files via presigned URL."
  value       = aws_s3_bucket.uploads.id
}

output "uploads_bucket_arn" {
  description = "ARN of the uploads bucket."
  value       = aws_s3_bucket.uploads.arn
}
