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

keyset.addPermission(pubkey, "expose", "server.example.com:22", "allow")

tokenset := import("tokenset")
oldTokens := tokenset.listActive("server1")
// revoke all old tokens for server1
for tk in oldTokens {
    tokenset.revoke(tk.ID)
}
token := tokenset.issue("server1", "Server 1 - API Token", "8766h")
echo.printJSON({
    pubkey: pubkey,
    apiToken: token
})

EOF
} | {
    {
        ssh -o LogLevel=QUIET ${jump_server} -- vandrare gateway ssh admin | tee output.json
        jq < output.json
    } || echo "Fail!"
}

ssh -o LogLevel=QUIET ${jump_server} -i "${priv_key_file}" -- whoami