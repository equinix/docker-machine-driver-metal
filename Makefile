default: build

mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
current_dir := $(notdir $(patsubst %/,%,$(dir $(mkfile_path))))
github_user := "packethost"
project := "github.com/$(github_user)/$(current_dir)"
bin_suffix := ""

containerbuild:
	docker build -t $(current_dir) .
	docker run \
		-v $(shell pwd):/go/src/$(project) \
		-e GOOS \
		-e GOARCH \
		-e GO15VENDOREXPERIMENT=1 \
		$(current_dir) \
		make build

clean:
	rm -r docker-machine-driver-equinix-metal bin/docker-machine-driver-equinix-metal

compile:
	GO111MODULE=on GOGC=off CGOENABLED=0 go build -ldflags "-s"

# deprecated in favor of goreleaser
pack: cross
	find ./bin -mindepth 1 -type d -exec zip -r -j {}.zip {} \;

# deprecated in favor of goreleaser
checksums: pack
	for file in $(shell find bin -type f -name '*.zip'); do \
		( \
		cd $$(dirname $$file); \
		f=$$(basename $$file); \
		b2sum     --tag $$f && \
		sha256sum --tag $$f && \
		sha512sum --tag $$f ; \
		) \
	done | sort >$@.tmp
	@mv $@.tmp $@

print-success:
	@echo
	@echo "Plugin built."
	@echo
	@echo "To use it, either run 'make install' or set your PATH environment variable correctly."

build: compile print-success

# deprecated in favor of goreleaser
cross:
	for os in darwin windows linux; do \
		for arch in amd64; do \
			GOOS=$$os GOARCH=$$arch BIN_SUFFIX=_$$os-$$arch $(MAKE) compile & \
		done; \
	done; \
	wait

install:
	cp bin/$(current_dir)/$(current_dir) /usr/local/bin/$(current_dir)

tag:
	if ! git tag | grep -q $(version); then \
		git tag -m $(version) $(version); \
		git push --tags; \
	fi
