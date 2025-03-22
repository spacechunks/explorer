.PHONY: cp
cp:
	POSTGRES_DSN=postgresql://spc:ouiwef98245z837@localhost:5432/explorer?sslmode=disable \
	LISTEN_ADDRESS=127.0.0.1:9012 \
	./controlplane/controlplane

.PHONY: pd
pd:
	./platformd/platformd