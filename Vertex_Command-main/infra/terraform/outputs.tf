output "ecr_repository_url" {
  description = "Push the engine image here (linux/arm64)."
  value       = aws_ecr_repository.engine.repository_url
}

output "ecr_registry" {
  description = "Registry host used by the docker ECR credential helper on the instances."
  value       = local.ecr_registry
}

output "instances" {
  description = "Per-region instance id, public IP, and SSH command."
  value = {
    us-east-1 = {
      instance_id = module.use1.instance_id
      public_ip   = module.use1.public_ip
      ssh         = "ssh -p ${var.ssh_port} vertex@${module.use1.public_ip}"
    }
    eu-central-1 = {
      instance_id = module.euc1.instance_id
      public_ip   = module.euc1.public_ip
      ssh         = "ssh -p ${var.ssh_port} vertex@${module.euc1.public_ip}"
    }
    ap-northeast-1 = {
      instance_id = module.apne1.instance_id
      public_ip   = module.apne1.public_ip
      ssh         = "ssh -p ${var.ssh_port} vertex@${module.apne1.public_ip}"
    }
  }
}
