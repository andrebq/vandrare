#!/usr/bin/env bash

set -eou pipefail

declare -r jump_server="${1}"
declare pub_key
pub_key=$(cat "${2}")
readonly pub_key

declare -r priv_key_file=${3}

{
cat - <<EOF
echo := import("echo")
keyset := import("keyset")
pubkey := "${pub_key}"
keyset.put(pubkey, "-1s", "8766h" /* one year */, "server.example.com")
echo.print("new key added to database", pubkey)

EOF
} | {
    ssh -o LogLevel=QUIET ${jump_server} -- vandrare gateway ssh admin || echo "Fail!"
}

ssh -o LogLevel=QUIET ${jump_server} -i "${priv_key_file}" -- whoami