WORKDIR := work
CNI_PLUGINS := $(WORKDIR)/plugins
IMG_TESTDATA_DIR := internal/image/testdata
TEST_IMG := $(IMG_TESTDATA_DIR)/img.tar.gz
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

functests: ARGS ?= ./test/functional/...
dbschema: export DATABASE_URL := $(DATABASE_URL)
testdb: export DATABASE_URL := $(DATABASE_URL)
dbgen: dbschema sqlc

.PHONY: goimports
formatimports:
	@goimports -d $(find . -type f -name '*.go' -not -path "./vendor/*")

.PHONY: setup
setup:
	$(RUN) $(SUDO) apt update
	$(RUN) $(SUDO) apt install -y linux-tools-common libbpf-dev
	$(RUN) $(SUDO) mount bpffs /sys/fs/bpf -t bpf

.PHONY: run-platformd
run-platformd: $(WORKDIR)
	$(RUN) go build -o ./$(WORKDIR)/platformd.bin ./cmd/platformd
	$(RUN) $(SUDO) ./$(WORKDIR)/platformd.bin

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

.PHONY: dev
dev:
	./dev/up.sh

.PHONY: unittests
unittests: $(TEST_IMG)
	$(RUN) go test $$(go list ./... | grep -v github.com/spacechunks/explorer/test/e2e \
                                    | grep -v github.com/spacechunks/explorer/test/functional) $(ARGS)

.PHONY: e2etests
e2etests:
	GOOS=linux GOARCH=arm64 go build -o ./dev/ptpnat ./cmd/ptpnat/main.go
	$(SUDO) go test ./test/e2e/...

.PHONY: functests-controlplane
functests-controlplane: $(TEST_IMG)
	$(RUN) $(SUDO) FUNCTESTS_POSTGRES_IMAGE=postgres:17 \
				   FUNCTESTS_POSTGRES_USER=spc \
				   FUNCTESTS_POSTGRES_PASS=test123 \
				   FUNCTESTS_POSTGRES_DB=explorer \
				   go test -v ./test/functional/controlplane $(ARGS)

.PHONY: functests-cni
functests-cni: $(CNI_PLUGINS)
	$(RUN) $(SUDO) CNI_PATH=$(shell pwd)/$(CNI_PLUGINS)/bin go test -v ./test/functional/cni

.PHONY: functests-database
functests-database:
	$(RUN) $(SUDO) FUNCTESTS_POSTGRES_IMAGE=postgres:17 \
				   FUNCTESTS_POSTGRES_USER=spc \
				   FUNCTESTS_POSTGRES_PASS=test123 \
				   FUNCTESTS_POSTGRES_DB=explorer \
				   go test -v ./test/functional/database $(ARGS)

.PHONY: functests-platformd
functests-platformd:
	$(RUN) $(SUDO) FUNCTESTS_ENVOY_IMAGE=docker.io/envoyproxy/envoy:v1.31.4 \
                   FUNCTESTS_ENVOY_CONFIG=../../../dev/platformd/envoy-xds.yaml \
				   go test -v ./test/functional/platformd $(ARGS)

.PHONY: functests-shared
functests-shared: $(TEST_IMG)
	$(RUN) $(SUDO) go test -v ./test/functional/shared

$(TEST_IMG):
	@docker build -t test-img -f $(IMG_TESTDATA_DIR)/Dockerfile $(IMG_TESTDATA_DIR)
	@docker image save test-img > $(IMG_TESTDATA_DIR)/img.tar.gz

$(CNI_PLUGINS): $(WORKDIR)
	git clone https://github.com/containernetworking/plugins.git $(CNI_PLUGINS)
	$(CNI_PLUGINS)/build_linux.sh

$(WORKDIR):
	mkdir $(WORKDIR)
