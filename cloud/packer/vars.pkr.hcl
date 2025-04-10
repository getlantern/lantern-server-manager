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

variable "version" {
  type    = string
}

variable "sing_box_version" {
  type    = string
  default = "1.11.7"
}
