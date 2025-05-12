#!/bin/sh

set -e

echo ">>> Updating system and installing dependencies"
apt-get update && apt-get install -y git curl wget unzip python3 python3-yaml

ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

## Install Azure CLI
curl -sL https://aka.ms/InstallAzureCLIDeb | bash

## Install kubectl
curl -LO "https://dl.k8s.io/release/v1.24.0/bin/linux/${ARCH}/kubectl" 
chmod +x ./kubectl
mv ./kubectl /usr/local/bin/kubectl

# Install kubelogin
VERSION=$(curl -s https://api.github.com/repos/Azure/kubelogin/releases/latest | grep '"tag_name"' | cut -d '"' -f 4)
wget "https://github.com/Azure/kubelogin/releases/download/${VERSION}/kubelogin-linux-${ARCH}.zip"
unzip kubelogin-linux-${ARCH}.zip
ls
mv ./bin/linux_${ARCH}/kubelogin /usr/local/bin/
rm kubelogin-linux-${ARCH}.zip

# Install helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

