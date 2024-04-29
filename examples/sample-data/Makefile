.PHONY: gen-all gen-jumpserver gen-server1-example-com run-jumpserver
.SILENT: run-jumpserver

include Commons.mk

# gen-keys for all the different examples
gen-all: gen-jumpserver gen-server1-example-com
gen-jumpserver: $(PWD)/jumpserver_config
gen-server1-example-com: $(PWD)/server1_config

run-jumpserver: $(PWD)/jumpserver_config
	[[ -n "$(command)" ]] || { echo "Missing command='command line', please try again." 1>&2; exit 1; }
	ssh -F $(PWD)/jumpserver_config jumpserver -- $(command)

clean:
	rm -fv $(PWD)/server1_*
	rm -fv $(PWD)/ca.pub
	rm -fv $(PWD)/*_config

$(PWD)/jumpserver_config:
	$(vandrare) ssh config \
		--endpoint $(endpoint) \
		--allow-http \
			jump \
				--alias $(jumpserverAlias) \
				--jump-server '$(jumpserverSSHHostport)' \
				--gateway-pubkey $(PWD)/ca.pub | tee $@
	ssh -F $@ jumpserver -- vandrare whoami

$(PWD)/ca.pub: $(PWD)/jumpserver_config

$(PWD)/server1_config: $(PWD)/ca.pub $(PWD)/server1_ed25519 $(PWD)/jumpserver_config
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

$(PWD)/server1_ed25519:
	ssh-keygen -t ed25519 -f $@ -q -N "" -C "bind to server1"