vandrare?=../../dist/vandrare

jumpserverAlias=jumpserver
jumpserverAddr?=127.0.0.1
jumpserverPort?=2222
jumpserverHTTPPort?=8222
endpoint?=http://$(jumpserverAddr):$(jumpserverHTTPPort)
identityFile?=~/.ssh/id_ed25519

jumpserverHostport?=$(jumpserverAddr):$(jumpserverPort)
jumpserverSSHHostport?=[$(jumpserverAddr)]:$(jumpserverPort)