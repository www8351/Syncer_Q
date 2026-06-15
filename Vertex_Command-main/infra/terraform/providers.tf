# Remote state lives in S3 + DynamoDB (created by ./bootstrap first).
# Initialize with:  terraform init -backend-config=backend.hcl
terraform {
  backend "s3" {}
}

# Default provider = us-east-1. ECR + IAM (global) are created here.
provider "aws" {
  region = "us-east-1"
  default_tags {
    tags = local.common_tags
  }
}

provider "aws" {
  alias  = "euc1"
  region = "eu-central-1"
  default_tags {
    tags = local.common_tags
  }
}

provider "aws" {
  alias  = "apne1"
  region = "ap-northeast-1"
  default_tags {
    tags = local.common_tags
  }
}
