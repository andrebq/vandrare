#!/usr/bin/env bash

set -eou pipefail

declare -r endpoint="${1}"
declare -r sshHost="${2}"
declare -r sshPort="${3}"
declare -r identity_file="${4}"

function show-host-fingerprint {
    declare -r prefix="${1}"
    declare -r output_file="${2}"
    declare fingerprint
    fingerprint=$(curl -qfsSL "${prefix}/gateway/ssh/certificates/hosts/gateway_known_hosts")

    cat - <<EOF
Attention!

Certificate obtained from upstream server is:

${fingerprint}
EOF
    cat > ${output_file} <<<"${fingerprint}"
}

function generate-temporary-config {
    declare -r config_file="${1}"
    declare -r ca_known_hosts="${2}"

    cat - > "${config_file}" <<EOF
Host vandrare-gateway
	User admin
	Hostname ${sshHost}
	Port ${sshPort}
	IdentitiesOnly yes
	IdentityFile ${identity_file}
	UserKnownHostsFile ${ca_known_hosts}
EOF
}

function generate-initial-token {
    declare -r user="${1}"
    declare -r token_file="${2}"
    declare -r ssh_config="${3}"

    # not having +e handles read returning non-zero
    set +e
IFS='' read -r -d '' command <<"EOF"
echo := import("echo")
tokenset := import("tokenset")

token := tokenset.issueLifetime("${user}", "${user} Token")
echo.printJSON({
    apiToken: token
})
EOF
    # let's handle errors properly again
    set -e
    readonly command

    ssh -F "${ssh_config}" vandrare-gateway -- vandrare gateway ssh admin <<<"${command}" | jq -r '.apiToken' | tee "${token_file}"
}

declare -r tmp_gateway_hosts_file="./vandrare-gateway-host.pub"
declare -r tmp_gateway_config="./vandrare-gateway.conf"

show-host-fingerprint "${endpoint}" "${tmp_gateway_hosts_file}"
generate-temporary-config "${tmp_gateway_config}" "${tmp_gateway_hosts_file}"
generate-initial-token "admin" "admin-token.secret" "${tmp_gateway_config}"
rm -rf "${tmp_gateway_hosts_file}" "${tmp_gateway_config}"