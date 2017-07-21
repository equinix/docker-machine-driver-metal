default: build

version := v0.1.3
version_description := "Docker Machine Driver Plugin to Provision on Packet"
human_name := "$(version) - Docker Machine v0.8.2+"
version := "$(version)"

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
	rm -r bin/docker-machine*

compile:
	GOGC=off CGOENABLED=0 go build -ldflags "-s" -o bin/$(current_dir)$(BIN_SUFFIX)/$(current_dir) bin/main.go

pack:
	find ./bin -mindepth 1 -type d -exec zip -r -j {}.zip {} \;

checksums:
	@for file in $(wildcard bin/*.zip); do \
		echo "sha256 $$(basename $$file)   $$(openssl dgst -sha256 < $$file)"; \
		echo "md5 $$(basename $$file)      $$(openssl dgst -md5 < $$file)"; \
	done

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
	cp bin/$(current_dir)/$(current_dir) /usr/local/bin/$(current_dir)

cleanrelease:
	github-release delete \
		--user $(github_user) \
		--repo $(current_dir) \
		--tag $(version)
	git tag -d $(version)
	git push origin :refs/tags/$(version)

release: cross pack checksums
	git tag -m $(version) $(version)
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
				--name $(current_dir)_$$os-$$arch.zip \
				--file bin/$(current_dir)_$$os-$$arch.zip; \
		done; \
	done
