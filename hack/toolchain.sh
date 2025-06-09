#!/bin/bash
ARCH="arm64"

LLVM_VERSION=18
sudo apt update
sudo apt install make
curl https://apt.llvm.org/llvm.sh > llvm
sudo bash llvm $LLVM_VERSION all
rm llvm

# platformd
mkdir /etc/platformd
sudo cp dev/platformd/config.json /etc/platformd/config.json
sudo cp dev/platformd/Corefile /etc/platformd/dns.conf
sudo cp dev/platformd/envoy-xds.yaml /etc/platformd/proxy.conf
sudo cp dev/platformd/crio.conf /etc/crio/crio.conf.d/99-nodedev.conf

cd /tmp

GO_VERSION=1.23.1
sudo wget https://go.dev/dl/go$GO_VERSION.linux-$ARCH.tar.gz
sudo tar -C /usr/local -xzf go$GO_VERSION.linux-$ARCH.tar.gz
rm go$GO_VERSION.linux-$ARCH.tar.gz
echo "export PATH=$PATH:/usr/local/go/bin" >> ~/.profile
source ~/.profile

# crictl
CRICTL_VERSION=v1.32.0 # check latest version in /releases page
wget https://github.com/kubernetes-sigs/cri-tools/releases/download/$CRICTL_VERSION/crictl-$CRICTL_VERSION-linux-$ARCH.tar.gz
sudo tar zxvf crictl-$CRICTL_VERSION-linux-$ARCH.tar.gz -C /usr/local/bin
rm -f crictl-$CRICTL_VERSION-linux-$ARCH.tar.gz

# cni plugins
git clone https://github.com/containernetworking/plugins.git
cd plugins
./build_linux.sh
cd -
sudo mkdir -p /opt/cni
sudo cp -r plugins/bin /opt/cni

# crio
CRIO_VERSION=1.32
sudo curl -fsSL https://pkgs.k8s.io/addons:/cri-o:/stable:/v$CRIO_VERSION/deb/Release.key | sudo gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg
sudo echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] https://pkgs.k8s.io/addons:/cri-o:/stable:/v$CRIO_VERSION/deb/ /" | sudo tee /etc/apt/sources.list.d/cri-o.list
sudo apt-get update
sudo apt-get install -y cri-o
sudo systemctl start crio.service
sudo sysctl -w net.ipv4.ip_forward=1
sudo sed -i 's/#net.ipv4.ip_forward=1/net.ipv4.ip_forward=1/' /etc/sysctl.conf # persist after reboot
sudo cp /etc/crio/policy.json /etc/containers/policy.json

# criu
CRIU_VERSION=v4.0
wget https://github.com/checkpoint-restore/criu/archive/refs/tags/$CRIU_VERSION.tar.gz
sudo tar -xzvf $CRIU_VERSION.tar.gz
export DEBIAN_FRONTEND=noninteractive
sudo apt install -y build-essential asciidoctor libprotobuf-dev
sudo apt install -y libprotobuf-c-dev protobuf-c-compiler protobuf-compiler
sudo apt install -y python3-protobuf pkg-config libbsd-dev
sudo apt install -y iproute2 libnftables-dev libgnutls28-dev
sudo apt install -y libnl-3-dev libnet-dev libcap-dev
cd criu-4.0
sudo make install
cd -

# crun (version shipped with crio does not support checkpointing)
# its also important to install after crio, otherwise we do not
# have checkpointing enabled.
git clone https://github.com/containers/crun.git
sudo apt-get install -y make git gcc build-essential pkgconf libtool \
   libsystemd-dev libprotobuf-c-dev libcap-dev libseccomp-dev libyajl-dev \
   go-md2man autoconf python3 automake
cd ./crun
sudo ./autogen.sh
sudo ./configure
sudo make
sudo mv crun /usr/bin/crun
cd -
