WORKDIR := work
CNI_PLUGINS := $(WORKDIR)/plugins
IMG_TESTDATA_DIR := internal/image/testdata
REPACK_IMG := internal/image/testdata/repack-img.tar.gz
UNPACK_IMG := internal/image/testdata/unpack-img.tar.gz
SUDO := sudo --preserve-env=PATH env
DATABASE_URL := postgres://postgres:test@localhost:5432/postgres?sslmode=disable
OS := $(shell uname)

ifeq ($(OS), Darwin)
	RUN := @limactl shell xcomp
else
	RUN := $(shell)
endif

define start_db
	@docker run --name testdb --rm -d -p 5432:5432 -e POSTGRES_PASSWORD=test postgres:17.2
	@cd controlplane && dbmate \
		--migrations-dir ./postgres/migrations \
		--schema-file ./postgres/schema.sql \
		--wait migrate \
		|| docker stop testdb
endef

dbschema: export DATABASE_URL := $(DATABASE_URL)
testdb: export DATABASE_URL := $(DATABASE_URL)

.PHONY: setup
setup:
	$(RUN) $(SUDO) apt update
	$(RUN) $(SUDO) apt install -y linux-tools-common libbpf-dev
	$(RUN) $(SUDO) mount bpffs /sys/fs/bpf -t bpf

.PHONY: sqlc
sqlc:
	@sqlc generate

.PHONY: dbschema
dbschema:
	$(call start_db)
	@docker stop testdb

.PHONY: testdb
testdb:
	$(call start_db)

.PHONY: testdb-rm
testdb-rm:
	@docker stop testdb

.PHONY: vmlinux
vmlinux:
	$(RUN) bpftool btf dump file /sys/kernel/btf/vmlinux format c > internal/datapath/bpf/include/vmlinux.h

.PHONY: gogen
gogen:
	$(RUN) go generate ./...

.PHONY: genproto
genproto:
	buf generate --template ./api/buf.gen.yaml --output ./api ./api

.PHONY: nodedev
nodedev:
	./nodedev/up.sh

.PHONY: unittests
unittests: $(REPACK_IMG) $(UNPACK_IMG)
	$(RUN) go test $$(go list ./... | grep -v github.com/spacechunks/explorer/test/e2e \
                                    | grep -v github.com/spacechunks/explorer/test/functional)

.PHONY: e2etests
e2etests:
	GOOS=linux GOARCH=arm64 go build -o ./nodedev/ptpnat ./cmd/ptpnat/main.go
	$(SUDO) go test ./test/e2e/...

.PHONY: functests
functests: $(CNI_PLUGINS)
	$(RUN) $(SUDO) FUNCTESTS_ENVOY_IMAGE=docker.io/envoyproxy/envoy:v1.31.4 \
				   FUNCTESTS_ENVOY_CONFIG=../../nodedev/platformd/envoy-xds.yaml \
				   CNI_PATH=$(shell pwd)/$(CNI_PLUGINS)/bin \
				   go test -v ./test/functional/...

$(REPACK_IMG):
	@docker build -t repack-img -f $(IMG_TESTDATA_DIR)/Dockerfile.repack $(IMG_TESTDATA_DIR)
	@docker image save repack-img > $(IMG_TESTDATA_DIR)/repack-img.tar.gz

$(UNPACK_IMG):
	@docker build -t unpack-img -f $(IMG_TESTDATA_DIR)/Dockerfile.unpack $(IMG_TESTDATA_DIR)
	@docker image save unpack-img > $(IMG_TESTDATA_DIR)/unpack-img.tar.gz

$(CNI_PLUGINS): $(WORKDIR)
	git clone https://github.com/containernetworking/plugins.git $(CNI_PLUGINS)
	$(CNI_PLUGINS)/build_linux.sh

$(WORKDIR):
	mkdir $(WORKDIR)
