packer {
  required_plugins {
    amazon = {
      source  = "github.com/hashicorp/amazon"
      version = "~> 1"
    }
  }
}

source "amazon-ebs" "amazon-linux" {
  ami_name      = "lantern-server-manager-${var.version}-{{timestamp}}"
  instance_type = "t2.micro"
  region        = var.aws_region
  source_ami_filter {
    filters = {
      name                = "*amzn3-ami-hvm-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["amazon"]
  }
  ssh_username = "ec2-user"
}

build {
  sources = ["source.amazon-ebs.amazon-linux"]

  provisioner "file" {
    source      = "../lantern-server-manager.service"
    destination = "/tmp/lantern-server-manager.service"
  }

  provisioner "shell" {
    inline = [
      "yum install -y qrencode",
      "curl -L https://github.com/SagerNet/sing-box/releases/download/v${var.sing_box_version}/sing-box-${var.sing_box_version}-linux-amd64.tar.gz -o /tmp/sing-box.tar.gz",
      "curl -L https://github.com/getlantern/lantern-server-manager/releases/download/v${var.version}/lantern-server-manager_${var.version}_linux_amd64.tar.gz -o /tmp/lantern-server-manager.tar.gz",
      "tar -xzf /tmp/lantern-server-manager.tar.gz -C /tmp",
      "tar -xzf /tmp/sing-box.tar.gz -C /tmp",
      "sudo mkdir -p /opt/lantern",
      "sudo mv /tmp/sing-box-${var.sing_box_version}-linux-amd64/sing-box /usr/local/bin/sing-box",
      "sudo mv /tmp/lantern-server-manager.service /opt/lantern/lantern-server-manager.service",
      "sudo mv /tmp/lantern-server-manager /opt/lantern/lantern-server-manager",
      "sudo systemctl enable /opt/lantern/lantern-server-manager.service",
      "rm /home/ec2-user/.ssh/authorized_keys",
      "sudo rm /root/.ssh/authorized_keys"
    ]
  }
}