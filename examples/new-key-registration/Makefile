include ../Commons.mk

keygen?=ssh-keygen
exampleKey?=./server.example.com_ed25519
examplePubKey=$(exampleKey).pub

run: $(examplePubKey)
	cd ../sample-data && $(MAKE) gen-all

	$(SHELL) run.sh $(jumpserverAlias) $(examplePubKey) $(exampleKey)

$(examplePubKey):
	ssh-keygen -t ed25519 -f $(exampleKey) -q -N "" -C "bind to server"