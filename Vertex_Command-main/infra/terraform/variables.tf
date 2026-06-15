variable "project" {
  description = "Name prefix for all resources."
  type        = string
  default     = "vertex-copy-engine"
}

variable "admin_cidr" {
  description = "Your public IP in CIDR form (e.g. 203.0.113.10/32) allowed to SSH. Do NOT use 0.0.0.0/0."
  type        = string
  validation {
    condition     = can(cidrhost(var.admin_cidr, 0)) && var.admin_cidr != "0.0.0.0/0"
    error_message = "admin_cidr must be a valid CIDR and must not be 0.0.0.0/0."
  }
}

variable "ssh_public_key" {
  description = "SSH public key to import (installed on the 'vertex' deploy user). e.g. 'ssh-ed25519 AAAA... you@host'."
  type        = string
}

variable "ssh_port" {
  description = "Hardened SSH port (matches infra/vps/provision.sh)."
  type        = number
  default     = 2222
}

variable "instance_type" {
  description = "Graviton/ARM64 instance type."
  type        = string
  default     = "c7g.medium"
}

variable "ecr_repo_name" {
  description = "ECR repository for the Go routing engine image."
  type        = string
  default     = "vertex-go-routing-engine"
}

variable "app_ingress_ports" {
  description = "TCP ports opened to 0.0.0.0/0 (interim; locked to Global Accelerator later)."
  type        = list(number)
  default     = [443]
}

variable "root_volume_gb" {
  description = "Root EBS volume size (GB)."
  type        = number
  default     = 30
}
