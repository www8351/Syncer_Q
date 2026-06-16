terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.60"
    }
  }
}

# Default VPC + subnets in this region (custom VPC deferred to a later step).
data "aws_vpc" "default" {
  default = true
}

data "aws_subnets" "default" {
  filter {
    name   = "vpc-id"
    values = [data.aws_vpc.default.id]
  }
}

# Ubuntu 24.04 LTS, ARM64, resolved per-region from Canonical's public SSM parameter.
data "aws_ssm_parameter" "ubuntu_arm64" {
  name = "/aws/service/canonical/ubuntu/server/24.04/stable/current/arm64/hvm/ebs-gp3/ami-id"
}

resource "aws_key_pair" "this" {
  key_name   = "${var.project}-${var.region_label}"
  public_key = var.ssh_public_key
}

resource "aws_security_group" "this" {
  name        = "${var.project}-${var.region_label}"
  description = "Vertex copy-trading engine host (${var.region_label})"
  vpc_id      = data.aws_vpc.default.id

  ingress {
    description = "SSH (hardened port, admin only)"
    from_port   = var.ssh_port
    to_port     = var.ssh_port
    protocol    = "tcp"
    cidr_blocks = [var.admin_cidr]
  }

  dynamic "ingress" {
    for_each = toset(var.app_ingress_ports)
    content {
      description = "App port ${ingress.value} (interim 0.0.0.0/0; lock to Global Accelerator later)"
      from_port   = ingress.value
      to_port     = ingress.value
      protocol    = "tcp"
      cidr_blocks = ["0.0.0.0/0"]
    }
  }

  egress {
    description = "All outbound (ECR pull, apt, AWS APIs)"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = { Name = "${var.project}-${var.region_label}" }
}

resource "aws_instance" "this" {
  ami                         = data.aws_ssm_parameter.ubuntu_arm64.value
  instance_type               = var.instance_type
  subnet_id                   = element(tolist(data.aws_subnets.default.ids), 0)
  vpc_security_group_ids      = [aws_security_group.this.id]
  key_name                    = aws_key_pair.this.key_name
  iam_instance_profile        = var.instance_profile
  associate_public_ip_address = true
  user_data_base64            = var.user_data_base64
  user_data_replace_on_change = true

  metadata_options {
    http_tokens   = "required" # IMDSv2 only
    http_endpoint = "enabled"
  }

  root_block_device {
    volume_type = "gp3"
    volume_size = var.root_volume_gb
    encrypted   = true
  }

  tags = { Name = "${var.project}-${var.region_label}" }
}
