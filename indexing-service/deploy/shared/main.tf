terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.86.0"
    }
  }
  backend "s3" {
    bucket  = "forge-terraform-state"
    key     = "forge/${var.app}/shared.tfstate"
    region  = "us-west-2"
    encrypt = true
  }
}

provider "aws" {
  allowed_account_ids = [var.allowed_account_id]
  region              = "us-west-2"
  default_tags {
    tags = {
      Environment  = "shared"
      ManagedBy    = "OpenTofu"
      Owner        = "forge"
      Team         = "Filecoin Foundation Engineering"
      Organization = "Filecoin Foundation"
      Project      = "${var.app}"
    }
  }
}

provider "aws" {
  alias               = "dev"
  allowed_account_ids = [var.allowed_account_id]
  region              = "us-east-2"
  default_tags {
    tags = {
      Environment  = "dev"
      ManagedBy    = "OpenTofu"
      Owner        = "forge"
      Team         = "Filecoin Foundation Engineering"
      Organization = "Filecoin Foundation"
      Project      = "${var.app}"
    }
  }
}

module "shared" {
  source = "github.com/storacha/storoku//shared?ref=v0.6.2"
  providers = {
    aws     = aws
    aws.dev = aws.dev
  }
  create_db                   = false
  caches                      = ["providers", "no-providers", "indexes", "claims", ]
  networks                    = ["warm", "forge", "test"]
  app                         = var.app
  create_shared_dev_resources = var.create_shared_dev_resources
  domain_base                 = var.domain_base
  setup_cloudflare            = false
}
