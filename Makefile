WORKDIR := work
CNI_PLUGINS := $(WORKDIR)/plugins
SUDO := sudo --preserve-env=PATH env
OS := $(shell uname)

ifeq ($(OS), Darwin)
	RUN := @limactl shell xcomp
else
	RUN := $(shell)
endif


.PHONY: setup
setup:
	$(RUN) $(SUDO) apt update
	$(RUN) $(SUDO) apt install -y linux-tools-common libbpf-dev
	$(RUN) $(SUDO) mount bpffs /sys/fs/bpf -t bpf

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

.PHONY: e2etests
e2etests:
	GOOS=linux GOARCH=arm64 go build -o ./nodedev/ptpnat ./cmd/ptpnat/main.go
	$(SUDO) go test ./test/e2e/...

functests: $(CNI_PLUGINS)
	$(RUN) $(SUDO) CNI_PATH=$(shell pwd)/$(CNI_PLUGINS)/bin go test -v ./test/functional/...

$(CNI_PLUGINS): $(WORKDIR)
	git clone git@github.com:containernetworking/plugins.git $(CNI_PLUGINS)
	$(CNI_PLUGINS)/build_linux.sh

$(WORKDIR):
	mkdir $(WORKDIR)
