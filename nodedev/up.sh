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

go build -o nodedev/netglue cmd/netglue/main.go \
  && go build -o nodedev/platformd cmd/platformd/main.go \
  && go build -o nodedev/test cmd/test/main.go

#if [ $RETEST == "true" ]; then
#  ip=$(hcloud server ip nodedev-yannic)
#  go build -o nodedev/netglue cmd/netglue/main.go \
#    && go build -o nodedev/platformd cmd/platformd/main.go \
#    && go build -o nodedev/test cmd/test/main.go
#  scp -r -o StrictHostKeyChecking=no nodedev/* root@$ip:/root
#  ssh -o StrictHostKeyChecking=no root@$ip 'cp netglue /opt/cni/bin/netglue'
#  exit 0
#fi

hcloud server delete nodedev-yannic
hcloud server create --name nodedev-yannic --type cax21 --image ubuntu-24.04 --ssh-key macos-m2-pro
ip=$(hcloud server ip nodedev-yannic)


sleep 30 # takes a bit of time until the server is reachable from the network

scp -r -o StrictHostKeyChecking=no nodedev/* root@$ip:/root
ssh -o StrictHostKeyChecking=no root@$ip '/root/provision-full.sh'
