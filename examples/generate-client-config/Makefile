vandrare?=../../dist/vandrare
endpoint?=http://localhost:8222
run:
	$(vandrare) ssh config \
		--endpoint $(endpoint) \
		--allow-http \
			client \
				--jump-alias jumpserver \
				--jump-server '[127.0.0.1]:2222' \
				--include-jump \
				--gateway-pubkey $(PWD)/ca.pub \
				--host server1.example.com | tee ./server1.example.com_sshconf
	
	cat $(PWD)/ca.pub
	curl http://localhost:8222/gateway/ssh/certificates/self > host-cert.pub
	ssh-keygen -L -f host-cert.pub
	ssh -vvvvF ./server1.example.com_sshconf jumpserver