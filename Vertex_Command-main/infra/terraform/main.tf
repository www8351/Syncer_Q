data "aws_caller_identity" "current" {}

locals {
  common_tags = {
    Project   = "vertex-copy-engine"
    ManagedBy = "terraform"
    Component = "copy-trading-engine"
  }

  ecr_registry = "${data.aws_caller_identity.current.account_id}.dkr.ecr.us-east-1.amazonaws.com"

  # provision.sh is embedded base64 (NOT templated) so its shell ${...}/$(...) are left
  # intact; only the small cloud-config vars below are interpolated. gzip+base64 keeps
  # user_data under the 16 KB EC2 limit.
  user_data_base64 = base64gzip(templatefile("${path.module}/templates/user_data.cloud-init.yaml.tftpl", {
    ssh_public_key = var.ssh_public_key
    ssh_port       = var.ssh_port
    ecr_registry   = local.ecr_registry
    provision_b64  = base64encode(file("${path.module}/../vps/provision.sh"))
  }))
}

module "use1" {
  source    = "./modules/region-host"
  providers = { aws = aws }

  region_label      = "use1"
  project           = var.project
  ssh_public_key    = var.ssh_public_key
  ssh_port          = var.ssh_port
  instance_type     = var.instance_type
  instance_profile  = aws_iam_instance_profile.ec2.name
  admin_cidr        = var.admin_cidr
  app_ingress_ports = var.app_ingress_ports
  root_volume_gb    = var.root_volume_gb
  user_data_base64  = local.user_data_base64
}

module "euc1" {
  source    = "./modules/region-host"
  providers = { aws = aws.euc1 }

  region_label      = "euc1"
  project           = var.project
  ssh_public_key    = var.ssh_public_key
  ssh_port          = var.ssh_port
  instance_type     = var.instance_type
  instance_profile  = aws_iam_instance_profile.ec2.name
  admin_cidr        = var.admin_cidr
  app_ingress_ports = var.app_ingress_ports
  root_volume_gb    = var.root_volume_gb
  user_data_base64  = local.user_data_base64
}

module "apne1" {
  source    = "./modules/region-host"
  providers = { aws = aws.apne1 }

  region_label      = "apne1"
  project           = var.project
  ssh_public_key    = var.ssh_public_key
  ssh_port          = var.ssh_port
  instance_type     = var.instance_type
  instance_profile  = aws_iam_instance_profile.ec2.name
  admin_cidr        = var.admin_cidr
  app_ingress_ports = var.app_ingress_ports
  root_volume_gb    = var.root_volume_gb
  user_data_base64  = local.user_data_base64
}
