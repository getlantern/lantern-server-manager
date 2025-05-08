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
    googlecompute = {
      source  = "github.com/hashicorp/googlecompute"
      version = "~> 1"
    }
  }
}

source "googlecompute" "ubuntu" {
  project_id = var.gcp_project_id
  source_image_family = "ubuntu-2204-lts"
  machine_type = "e2-micro"
  zone = var.gcp_zone
  ssh_username = "ubuntu"
  image_name = "lantern-server-manager-{{timestamp}}"
  network = "lanternet"
  subnetwork = "default"
  use_iap = true
}

source "digitalocean" "nyc1" {
  api_token    = "${var.do_api_token}"
  image        = "debian-12-x64"
  region       = "nyc1"
  size         = "s-1vcpu-1gb"
  ssh_username = "root"
  snapshot_name = "lantern-server-manager-{{timestamp}}"
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
    # "source.digitalocean.nyc1",
    # "source.googlecompute.ubuntu",
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
    only = ["source.digitalocean.ubuntu-nyc1"]
    inline = [
      "journalctl --verify",
      "systemctl restart systemd-journald.service",
    ]
  }

  provisioner "shell" {
    only = ["source.amazon-ebs.amazon-linux"]
    inline = [
      "sudo rm -f /root/.ssh/authorized_keys",
      "sudo rm -f /home/ec2-user/.ssh/authorized_keys"
    ]
  }
}