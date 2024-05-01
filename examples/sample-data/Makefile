.PHONY: gen-all gen-jumpserver gen-server1-example-com run-jumpserver init
.SILENT: run-jumpserver ./InitialToken.mk ./jumpserver_config ./server1_config init
token?=$(shell cat ../sample-data/admin-token.secret || true)

include ../Commons.mk

# gen-keys for all the different examples
gen-all: gen-jumpserver gen-server1-example-com
gen-jumpserver: ./jumpserver_config
gen-server1-example-com: ./server1_config

run-jumpserver: ./jumpserver_config
	[[ -n "$(command)" ]] || { echo "Missing command='command line', please try again." 1>&2; exit 1; }
	ssh -F ./jumpserver_config jumpserver -- $(command)

init:
	[[ -f $(identityFile) ]] || { echo "Missing $(identityFile) please create a valid ssh key and try again" 1>&2; exit 1; }
	bash initial-token.sh "$(endpoint)" "$(jumpserverAddr)" "$(jumpserverPort)" "$(identityFile)"

$(identityFile):
	echo "Missing $(identityFile), please create a new ssh ed25519 key and try again"

clean:
	rm -fv ./server1_*
	rm -fv ./ca.pub
	rm -fv ./*_config

jumpserver_config:
	VANDRARE_GATEWAY_SSH_CONFIG_TOKEN=$(token) \
		$(vandrare) ssh config \
			--endpoint $(endpoint) \
			--allow-http \
				jump \
					--alias $(jumpserverAlias) \
					--jump-server '$(jumpserverSSHHostport)' \
					--gateway-pubkey ./ca.pub | tee $@
	ssh -F $@ jumpserver -- vandrare whoami

ca.pub: ./jumpserver_config

server1_config: ca.pub server1_ed25519 jumpserver_config
	VANDRARE_GATEWAY_SSH_CONFIG_TOKEN=$(token) \
		$(vandrare) ssh config \
			--endpoint $(endpoint) \
			--allow-http \
				client \
					--jump-alias $(jumpserverAlias) \
					--jump-server '$(jumpserverSSHHostport)' \
					--include-jump \
					--gateway-pubkey $(word 1,$^) \
					--host server1.example.com \
					--identity-file $(word 2,$^) | tee $@

server1_ed25519:
	ssh-keygen -t ed25519 -f $@ -q -N "" -C "bind to server1"