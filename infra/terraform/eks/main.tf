###############################################################################
# DevSpeak AI — EKS migration module
#
# Módulo isolado. Usa state próprio (não compartilha com a EC2 atual).
# Provisiona: VPC + EKS cluster + managed node group + ECR pull policy.
#
# ATENÇÃO: aplicar isso vai criar recursos pagos na AWS:
#   - 1 EKS control plane (~$73/mês = $0.10/h)
#   - 2 EC2 t3.small nodes (~$30/mês cada)
#   - 1 NAT gateway (~$32/mês + tráfego)
#   Total estimado: ~$165/mês mínimo.
#
# Uso:
#   $ cd infra/terraform/eks
#   $ terraform init
#   $ terraform plan
#   $ terraform apply
#
# Pós-apply:
#   $ aws eks update-kubeconfig --region us-east-1 --name devspeak-eks
#   $ kubectl get nodes
#   $ kubectl apply -f ../../k8s/
###############################################################################

terraform {
  required_version = ">= 1.6"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

variable "region" {
  type    = string
  default = "us-east-1"
}

variable "cluster_name" {
  type    = string
  default = "devspeak-eks"
}

variable "node_instance_type" {
  type    = string
  default = "t3.small"
}

variable "node_desired_size" {
  type    = number
  default = 2
}

variable "node_max_size" {
  type    = number
  default = 4
}

# ── VPC pequena, 2 AZs, com NAT pra subnets privadas ──────────────────────────

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 5.8"

  name = "${var.cluster_name}-vpc"
  cidr = "10.30.0.0/16"

  azs             = ["${var.region}a", "${var.region}b"]
  public_subnets  = ["10.30.0.0/20", "10.30.16.0/20"]
  private_subnets = ["10.30.32.0/20", "10.30.48.0/20"]

  enable_nat_gateway   = true
  single_nat_gateway   = true # economiza ~$32/mês ao custo de zero-redundância
  enable_dns_hostnames = true

  # tags exigidas pelo EKS pra descobrir as subnets
  public_subnet_tags = {
    "kubernetes.io/role/elb"                      = 1
    "kubernetes.io/cluster/${var.cluster_name}"   = "shared"
  }
  private_subnet_tags = {
    "kubernetes.io/role/internal-elb"             = 1
    "kubernetes.io/cluster/${var.cluster_name}"   = "shared"
  }
}

# ── Cluster EKS gerenciado ────────────────────────────────────────────────────

module "eks" {
  source  = "terraform-aws-modules/eks/aws"
  version = "~> 20.20"

  cluster_name    = var.cluster_name
  cluster_version = "1.30"

  cluster_endpoint_public_access = true

  vpc_id     = module.vpc.vpc_id
  subnet_ids = module.vpc.private_subnets

  enable_irsa = true

  eks_managed_node_groups = {
    workers = {
      ami_type       = "AL2_x86_64"
      instance_types = [var.node_instance_type]
      min_size       = 1
      desired_size   = var.node_desired_size
      max_size       = var.node_max_size

      # speech-service precisa puxar imagem do GHCR — IAM role do node tem
      # acesso de saída via NAT, então pull funciona sem config extra.
    }
  }

  cluster_addons = {
    coredns = { most_recent = true }
    kube-proxy = { most_recent = true }
    vpc-cni = { most_recent = true }
  }
}

# ── Outputs ───────────────────────────────────────────────────────────────────

output "cluster_name" {
  value = module.eks.cluster_name
}

output "kubeconfig_command" {
  value = "aws eks update-kubeconfig --region ${var.region} --name ${module.eks.cluster_name}"
}

output "cluster_endpoint" {
  value = module.eks.cluster_endpoint
}
