locals {
  uploads_bucket_name = "${local.name_prefix}-uploads-${local.account_id}"
}

resource "aws_s3_bucket" "uploads" {
  bucket = local.uploads_bucket_name
}

resource "aws_s3_bucket_public_access_block" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_lifecycle_configuration" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  rule {
    id     = "expire-uploads"
    status = "Enabled"

    filter {
      prefix = "uploads/"
    }

    expiration {
      days = 30
    }
  }
}

# CORS open to any origin in MVP. Tighten to the CloudFront domain once the
# frontend distribution is in place.
resource "aws_s3_bucket_cors_configuration" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  cors_rule {
    allowed_methods = ["PUT"]
    allowed_origins = ["*"]
    allowed_headers = ["*"]
    max_age_seconds = 3000
  }
}

data "aws_iam_policy_document" "uploads" {
  statement {
    sid    = "DenyNonTLS"
    effect = "Deny"

    principals {
      type        = "*"
      identifiers = ["*"]
    }

    actions = ["s3:*"]

    resources = [
      aws_s3_bucket.uploads.arn,
      "${aws_s3_bucket.uploads.arn}/*",
    ]

    condition {
      test     = "Bool"
      variable = "aws:SecureTransport"
      values   = ["false"]
    }
  }

  # Grants the shared Lambda execution role read and write access on the
  # uploads/ prefix. Needed because the role itself has no S3 permissions
  # in the lab environment, so we authorise it from the resource side.
  statement {
    sid    = "AllowLambdaUploadsAccess"
    effect = "Allow"

    principals {
      type        = "AWS"
      identifiers = [local.lambda_execution_role_arn]
    }

    actions = [
      "s3:GetObject",
      "s3:PutObject",
    ]

    resources = ["${aws_s3_bucket.uploads.arn}/uploads/*"]
  }
}

resource "aws_s3_bucket_policy" "uploads" {
  bucket = aws_s3_bucket.uploads.id
  policy = data.aws_iam_policy_document.uploads.json
}
