locals {
  web_bucket_name = "${local.name_prefix}-web-${local.account_id}"
}

resource "aws_s3_bucket" "web" {
  bucket = local.web_bucket_name
}

resource "aws_s3_bucket_public_access_block" "web" {
  bucket = aws_s3_bucket.web.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_server_side_encryption_configuration" "web" {
  bucket = aws_s3_bucket.web.id

  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

# Origin Access Control replaces the legacy OAI. CloudFront signs requests
# to S3 with SigV4 so the bucket can stay private.
resource "aws_cloudfront_origin_access_control" "web" {
  name                              = "${local.name_prefix}-web"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_distribution" "web" {
  enabled             = true
  default_root_object = "index.html"
  is_ipv6_enabled     = true
  price_class         = "PriceClass_100"
  comment             = local.name_prefix

  origin {
    origin_id                = "web-s3"
    domain_name              = aws_s3_bucket.web.bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.web.id
  }

  default_cache_behavior {
    target_origin_id       = "web-s3"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD"]
    cached_methods         = ["GET", "HEAD"]
    compress               = true

    min_ttl     = 0
    default_ttl = 300
    max_ttl     = 3600

    forwarded_values {
      query_string = false
      cookies {
        forward = "none"
      }
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }
}

# Allow the CloudFront distribution to read objects from the web bucket.
# aws:SourceArn pins this to our distribution only.
data "aws_iam_policy_document" "web" {
  statement {
    sid    = "AllowCloudFrontReadOnly"
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["cloudfront.amazonaws.com"]
    }

    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.web.arn}/*"]

    condition {
      test     = "StringEquals"
      variable = "AWS:SourceArn"
      values   = [aws_cloudfront_distribution.web.arn]
    }
  }
}

resource "aws_s3_bucket_policy" "web" {
  bucket = aws_s3_bucket.web.id
  policy = data.aws_iam_policy_document.web.json
}

locals {
  index_html = templatefile("${path.module}/../web/index.html", {
    api_endpoint      = aws_apigatewayv2_stage.default.invoke_url
    cognito_domain    = "https://${aws_cognito_user_pool_domain.users.domain}.auth.${var.region}.amazoncognito.com"
    cognito_client_id = aws_cognito_user_pool_client.web.id
    redirect_uri      = "https://${aws_cloudfront_distribution.web.domain_name}"
  })
}

resource "aws_s3_object" "index_html" {
  bucket       = aws_s3_bucket.web.id
  key          = "index.html"
  content      = local.index_html
  content_type = "text/html; charset=utf-8"
  etag         = md5(local.index_html)
}
