variable "aws_region" {
  default = "us-east-1"
}

variable "aws_secret_key" {
  type      = string
  sensitive = true
}

variable "aws_access_key" {
  type      = string
  sensitive = true
}

variable "do_api_token" {
  type      = string
  sensitive = true
}

variable "gcp_project_id" {
  type = string
}

variable "gcp_zone" {
  type = string
}