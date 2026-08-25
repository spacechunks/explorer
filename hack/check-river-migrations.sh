#!/usr/bin/env bash

curr=$(grep -rh "River main migration" controlplane/postgres/migrations  | tail -1 | grep -o "[1-999]")
new=$(go tool river migrate-get --all --up | grep -rh "River main migration" | tail -1 | grep -o "[1-999]")
[ "$curr" = "$new" ] || { echo "found new river migration: current is '$curr', new is '$new'" >&2; exit 1; }
