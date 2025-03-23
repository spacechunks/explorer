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

export GOOS=linux
export GOARCH=arm64

go build -o dev/cni/netglue cmd/netglue/main.go \
  && go build -o dev/platformd/platformd cmd/platformd/main.go \
  && go build -o dev/test cmd/test/main.go \
  && go build -o dev/controlplane/controlplane cmd/controlplane/main.go

#if [ $RETEST == "true" ]; then
#  ip=$(hcloud server ip dev-yannic)
#  go build -o dev/cni/netglue cmd/netglue/main.go \
#    && go build -o dev/platformd/platformd cmd/platformd/main.go \
#    && go build -o dev/test cmd/test/main.go \
#    && go build -o dev/controlplane/controlplane cmd/controlplane/main.go
#  scp -r -o StrictHostKeyChecking=no dev/* root@$ip:/root
#  ssh -o StrictHostKeyChecking=no root@$ip 'cp /root/cni/netglue /opt/cni/bin/netglue'
#  ssh -o StrictHostKeyChecking=no root@$ip 'cp /root/platformd/config.json /etc/platformd/config.json'
#  exit 0
#fi

hcloud server delete dev-yannic
hcloud server create --name dev-yannic --type cax21 --image ubuntu-24.04 --ssh-key macos-m2-pro
ip=$(hcloud server ip dev-yannic)


sleep 30 # takes a bit of time until the server is reachable from the network

scp -r -o StrictHostKeyChecking=no dev/* root@$ip:/root
ssh -o StrictHostKeyChecking=no root@$ip '/root/provision-full.sh'
