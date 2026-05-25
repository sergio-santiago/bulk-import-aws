resource "aws_cognito_user_pool" "users" {
  name = "${local.name_prefix}-users"

  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]

  password_policy {
    minimum_length    = 8
    require_lowercase = true
    require_uppercase = true
    require_numbers   = true
    require_symbols   = false
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }

  # Default Cognito email sender. Caps at 50 emails/day, enough for the
  # MVP demo. For production we would wire up SES with a verified domain.
  email_configuration {
    email_sending_account = "COGNITO_DEFAULT"
  }
}

resource "aws_cognito_user_pool_domain" "users" {
  # Cognito disallows the word "aws" in domain prefixes, hence the
  # explicit value rather than reusing local.name_prefix.
  domain       = "bulk-import-${local.account_id}"
  user_pool_id = aws_cognito_user_pool.users.id
}

resource "aws_cognito_user_pool_client" "web" {
  name         = "${local.name_prefix}-web"
  user_pool_id = aws_cognito_user_pool.users.id

  generate_secret = false

  explicit_auth_flows = [
    "ALLOW_USER_SRP_AUTH",
    "ALLOW_REFRESH_TOKEN_AUTH",
  ]

  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email"]
  allowed_oauth_flows_user_pool_client = true
  supported_identity_providers         = ["COGNITO"]

  # Placeholder while the CloudFront distribution does not exist. Updated
  # in the frontend layer with the real distribution domain.
  callback_urls = ["https://localhost"]
  logout_urls   = ["https://localhost"]
}
