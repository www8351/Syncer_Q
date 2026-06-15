variable "region_label" {
  description = "Short region label used in resource names (e.g. use1, euc1, apne1)."
  type        = string
}

variable "project" {
  type = string
}

variable "ssh_public_key" {
  type = string
}

variable "ssh_port" {
  type = number
}

variable "instance_type" {
  type = string
}

variable "instance_profile" {
  description = "Name of the (global) IAM instance profile granting ECR read-only."
  type        = string
}

variable "admin_cidr" {
  type = string
}

variable "app_ingress_ports" {
  type = list(number)
}

variable "root_volume_gb" {
  type = number
}

variable "user_data_base64" {
  description = "gzip+base64 cloud-config (provisioning + hardening + ECR cred helper)."
  type        = string
}
