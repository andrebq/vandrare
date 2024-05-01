.PHONY: run
run:
	mkdir -p $(keyStoreDir)
	VANDRARE_GATEWAY_SSH_CA_SEED=$(GATEWAY_SSH_CA_SEED) \
		./dist/vandrare \
			--log-level debug \
			ssh gateway \
				--admin-key-file $(adminKey) \
				--keydb-store-dir $(keyStoreDir) \
				--self-domain "127.0.0.1:2222" \
				--domain example.com