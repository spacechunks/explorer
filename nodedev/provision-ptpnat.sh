#!/bin/bash

# Explorer Platform, a platform for hosting and discovering Minecraft servers.
# Copyright (C) 2024 Yannic Rieger <oss@76k.io>
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU Affero General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Affero General Public License for more details.
#
# You should have received a copy of the GNU Affero General Public License
# along with this program.  If not, see <https://www.gnu.org/licenses/>.

apt update

# linux-tools-6.8.0-38-generic needed for bpftool
apt-get install -y gnupg2 git linux-tools-6.8.0-38-generic

# pwru
wget https://github.com/cilium/pwru/releases/download/v1.0.8/pwru-linux-arm64.tar.gz
tar -xzvf pwru-linux-arm64.tar.gz

# go
wget https://go.dev/dl/go1.23.1.linux-arm64.tar.gz
tar -C /usr/local -xzf go1.23.1.linux-arm64.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.profile

# cni plugins
git clone https://github.com/containernetworking/plugins.git
cd plugins
./build_linux.sh
cd -
mkdir -p /opt/cni
cp -r plugins/bin /opt/cni

# install ptpnat
cp ptpnat /opt/cni/bin/ptpnat
mkdir -p /etc/cni/net.d/
cp /root/10-ptpnat.conflist /etc/cni/net.d/10-ptpnat.conflist

# crio
MAJOR_VERSION=1.30
curl -fsSL https://pkgs.k8s.io/addons:/cri-o:/stable:/v$MAJOR_VERSION/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] https://pkgs.k8s.io/addons:/cri-o:/stable:/v$MAJOR_VERSION/deb/ /" | tee /etc/apt/sources.list.d/cri-o.list
apt-get update
apt-get install -y cri-o
systemctl start crio.service
sysctl -w net.ipv4.ip_forward=1
sed -i 's/#net.ipv4.ip_forward=1/net.ipv4.ip_forward=1/' /etc/sysctl.conf # persist after reboot

# crictl
VERSION=v1.30.1 # check latest version in /releases page
ARCH=arm64
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$VERSION/crictl-$VERSION-linux-$ARCH.tar.gz
sudo tar zxvf crictl-$VERSION-linux-$ARCH.tar.gz -C /usr/local/bin
rm -f crictl-$VERSION-linux-$ARCH.tar.gz

cp ./10-ignore.link /etc/systemd/network/10-ignore.link
systemctl restart systemd-networkd
