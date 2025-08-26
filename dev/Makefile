.PHONY: cli
cli:
	go build -o cli/explorer ../cmd/cli/main.go
	cd cli && ./explorer chunk publish

.PHONY: cp
cp:
	go build -o controlplane/controlplane ../cmd/controlplane/main.go
	(sleep 3 && PGPASSWORD=ouiwef98245z837 psql -h localhost --username=spc -d explorer -c "INSERT INTO nodes (id, name, address, checkpoint_api_endpoint) VALUES ('0195c2f6-f40c-72df-a0f1-e468f1be77b1', 'local', '127.0.0.1', '127.0.0.1:3011')") &
	CONTROLPLANE_POSTGRES_DSN=postgres://spc:ouiwef98245z837@localhost:5432/explorer?sslmode=disable \
	CONTROLPLANE_LISTEN_ADDRESS=127.0.0.1:9012 \
	CONTROLPLANE_GRPC_MAX_MESSAGE_SIZE=1000000000 \
	CONTROLPLANE_BASE_IMAGE=ghcr.io/spacechunks/paper-docker:1.21.8-1 \
	CONTROLPLANE_OCI_REGISTRY=ghcr.io/spacechunks/explorer-dev \
	CONTROLPLANE_OCI_REGISTRY_USER="" \
	CONTROLPLANE_OCI_REGISTRY_PASS="" \
	CONTROLPLANE_IMAGE_PLATFORM=linux/arm64 \
	./controlplane/controlplane

.PHONY: pd
pd:
	go build -o platformd/platformd ../cmd/platformd/main.go
	sudo PLATFORMD_ENABLE_CRIO_RESTART=true ./platformd/platformd

.PHONY: upload-pd
upload-pd:
	@limactl shell xcomp GOOS=linux GOARCH=arm64 go build -o platformd/platformd ../cmd/platformd/main.go
	scp platformd/platformd root@$(PLATFORMD_STAGE_HOST):/usr/bin/platformd
	#scp platformd/platformd.service root@$(PLATFORMD_STAGE_HOST):/lib/systemd/system/platformd.service

.PHONY: t
t:
	./test