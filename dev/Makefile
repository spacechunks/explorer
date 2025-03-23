.PHONY: cp
cp:
	CONTROLPLANE_POSTGRES_DSN=postgres://spc:ouiwef98245z837@localhost:5432/explorer?sslmode=disable \
	CONTROLPLANE_LISTEN_ADDRESS=127.0.0.1:9012 \
	./controlplane/controlplane

.PHONY: pd
pd:
	PLATFORMD_ENABLE_CRIO_RESTART=true ./platformd/platformd

.PHONY: t
t:
	./test