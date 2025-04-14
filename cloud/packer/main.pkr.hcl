packer {
  required_plugins {
    amazon = {
      source  = "github.com/hashicorp/amazon"
      version = "~> 1"
    }
    ansible = {
      source  = "github.com/hashicorp/ansible"
      version = "~> 1"
    }
    digitalocean = {
      source  = "github.com/hashicorp/digitalocean"
      version = "~> 1"
    }
  }
}

source "digitalocean" "ubuntu" {
  api_token    = "${var.do_api_token}"
  image        = "ubuntu-24-10-x64"
  region       = "nyc3"
  size         = "s-1vcpu-1gb"
  ssh_username = "root"
}

source "amazon-ebs" "amazon-linux" {
  ami_name      = "lantern-server-manager-{{timestamp}}"
  instance_type = "t2.micro"
  region        = var.aws_region
  source_ami_filter {
    filters = {
      name                = "al2023-ami*x86_64"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["amazon"]
  }
  ssh_username = "ec2-user"
}

build {
  sources = [
    "source.amazon-ebs.amazon-linux",
    "source.digitalocean.ubuntu"
  ]

  provisioner "shell-local" {
    command = "ansible-galaxy collection install ansible.posix"
  }

  provisioner "ansible" {
    playbook_file = "./playbook.yml"
    use_proxy = false
    extra_arguments = [
      "--extra-vars",
      "ansible_user=${build.User}",
    ]
  }

  provisioner "shell" {
    inline = [
      "sudo systemctl restart systemd-journald.service",
      "sudo rm -f /root/.ssh/authorized_keys",
      "sudo rm -f /home/ec2-user/.ssh/authorized_keys"
    ]
  }
}