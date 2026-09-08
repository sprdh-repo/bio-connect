#!/bin/bash
# Lightsail user-data for the bioconnect-registration instance.
# Pass with:  aws lightsail create-instances ... --user-data "$(cat ops/cloud-init.sh)"
# NOTE: Lightsail runs this under /bin/sh in some images - keep it POSIX, no `set -o pipefail`.
set -eux
export DEBIAN_FRONTEND=noninteractive

# Swap: small_3_1 has 2 GB RAM; PostgreSQL + Go + Caddy + occasional PDF/XLSX work benefits.
if [ ! -f /swapfile ]; then
  fallocate -l 2G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  echo '/swapfile none swap sw 0 0' >> /etc/fstab
  echo 'vm.swappiness=10' > /etc/sysctl.d/99-swap.conf
  sysctl -w vm.swappiness=10
fi

apt-get update -y
apt-get install -y ca-certificates curl gnupg
install -m 0755 -d /etc/apt/keyrings
if [ ! -f /etc/apt/keyrings/docker.asc ]; then
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
fi
. /etc/os-release
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" > /etc/apt/sources.list.d/docker.list
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin unattended-upgrades awscli
systemctl enable --now docker
usermod -aG docker ubuntu

mkdir -p /opt/bioconnect/ops /etc/bioconnect /var/lib/bioconnect-backup
chmod 700 /etc/bioconnect /var/lib/bioconnect-backup
touch /opt/bioconnect/.provisioned
