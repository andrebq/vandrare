#!/usr/bin/env bash

set -eou pipefail

declare -r jump_server="${1}"
declare pub_key
pub_key=$(cat "${2}")
readonly pub_key

{
cat - <<EOF
(func(){
    keyset := import("keyset")

    keyset.put("${pub_key}", "-1s", "8766h" /* one year */, "server.example.com")
    return "${pub_key}"
}())
EOF
} | tee command.txt

ssh ${jump_server} -- vandrare gateway ssh admin < command.txt