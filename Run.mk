run:
	mkdir -p $(keyStoreDir)
	./dist/vandrare --log-level debug ssh gateway --admin-key-file $(adminKey) --keydb-store-dir $(keyStoreDir)