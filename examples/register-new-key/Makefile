keygen?=ssh-keygen

exampleKey?=./server.example.com_ed25519
examplePubKey=$(exampleKey).pub
serverHost?=jumpserver

run: $(examplePubKey)
	$(SHELL) run.sh $(serverHost) $(examplePubKey)

$(examplePubKey):
	ssh-keygen -t ed25519 -f $(exampleKey) -q -N "" -C "bind to server"