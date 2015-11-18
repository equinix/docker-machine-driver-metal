default: build

version := "v0.0.1"
version_description := "Docker Machine Driver Plugin to provision on Packet"
human_name := "Packet Driver"

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

containerrelease:
	docker build -t $(current_dir) .
	docker run \
		-v $(shell pwd):/go/src/$(project) \
		-e GOOS \
		-e GOARCH \
		-e GITHUB_TOKEN \
		-e GO15VENDOREXPERIMENT=1 \
		$(current_dir) \
		make release

clean:
	rm bin/docker-machine*

compile:
	GOGC=off CGOENABLED=0 go build -ldflags "-s" -o bin/$(current_dir)$(BIN_SUFFIX) bin/main.go

print-success:
	@echo
	@echo "Plugin built."
	@echo
	@echo "To use it, either run 'make install' or set your PATH environment variable correctly."

build: compile print-success

cross:
	for os in darwin windows linux; do \
		for arch in amd64; do \
			GOOS=$$os GOARCH=$$arch BIN_SUFFIX=_$$os-$$arch $(MAKE) compile & \
		done; \
	done; \
	wait

install:
	cp bin/$(current_dir) /usr/local/bin/$(current_dir)

cleanrelease:
	github-release delete \
		--user $(github_user) \
		--repo $(current_dir) \
		--tag $(version)
	git tag -d $(version)
	git push origin :refs/tags/$(version)

release: cross
	git tag $(version)
	git push --tags
	github-release release \
		--user $(github_user) \
		--repo $(current_dir) \
		--tag $(version) \
		--name $(human_name) \
		--description $(version_description)
	for os in darwin windows linux; do \
		for arch in amd64; do \
			github-release upload \
				--user $(github_user) \
				--repo $(current_dir) \
				--tag $(version) \
				--name bin/$(current_dir)_$$os-$$arch \
				--file bin/$(current_dir)_$$os-$$arch; \
		done; \
	done
