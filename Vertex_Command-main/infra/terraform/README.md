# Step 2 — AWS infra (ECR + 3-region Graviton compute)

Terraform for the copy-trading **Go routing engine**: one ECR registry + 3 hardened
ARM64 EC2 hosts (us-east-1, eu-central-1, ap-northeast-1) that pull the image keyless via
an IAM instance profile.

> ⚠️ Creates **billable** AWS resources (3× EC2 always-on, ECR, S3, DynamoDB). Run
> `terraform destroy` when done testing.

## What it creates
- `aws_ecr_repository` `vertex-go-routing-engine` (us-east-1, scan-on-push, immutable tags, keep-last-10).
- IAM role + instance profile with `AmazonEC2ContainerRegistryReadOnly` (global).
- Per region: `aws_key_pair` (your imported key), security group, 1 `c7g.medium` instance
  (Ubuntu 24.04 ARM64 via Canonical SSM), IMDSv2-only, encrypted gp3 root.
- cloud-init runs `infra/vps/provision.sh` (disable root login + password auth, SSH 2222,
  UFW, fail2ban, Docker + Compose, sysctl) and installs the ECR credential helper.

Security groups: `2222/tcp` ← `admin_cidr` only; `app_ingress_ports` (default `443`) ←
`0.0.0.0/0` (interim — locked to Global Accelerator in the next step).

## Prerequisites
AWS CLI configured (admin creds for apply), Terraform ≥ 1.6, Docker w/ buildx (for the image).

## 1. Bootstrap remote state (once)
```bash
cd infra/terraform/bootstrap
terraform init
terraform apply -var="state_bucket=vertex-tfstate-<unique-suffix>"
```

## 2. Configure
```bash
cd infra/terraform
cp backend.hcl.example backend.hcl            # set bucket = the one just created
cp terraform.tfvars.example terraform.tfvars  # set admin_cidr (your IP/32) + ssh_public_key
```

## 3. Apply
```bash
terraform init -backend-config=backend.hcl
terraform fmt -recursive && terraform validate
terraform plan      # expect: 1 ECR, 1 role+profile, 3 key pairs, 3 SGs, 3 instances
terraform apply
```
Outputs: `ecr_repository_url`, and per-region `public_ip` + ready-to-paste `ssh` command.

## 4. Build + push the engine image (ARM64)
```bash
AWS_REGION=us-east-1 TAG=v1 ../backend-go/build-push.sh
```

## 5. Verify (per host)
```bash
ssh -p 2222 vertex@<public_ip>
cloud-init status --wait            # done
uname -m                            # aarch64
docker --version && docker compose version
sudo ufw status                     # 2222, 80, 443
docker pull <ecr_repository_url>:v1 # keyless pull via instance role
ssh root@<public_ip>                # MUST be refused (root login disabled)
```

## Teardown
```bash
terraform destroy                                  # in infra/terraform
cd bootstrap && terraform destroy -var="state_bucket=..."   # last
```

## Notes
- Engine app-wiring (Redis endpoint, `WEBHOOK_SECRET`, running the container) is a later step.
- `terraform.tfvars` / `backend.hcl` hold your IP + key and are gitignored (`.example` committed).
