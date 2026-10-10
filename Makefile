# Copyright 2021 Linka Cloud  All rights reserved.
# Modifications copyright 2026 jimyag. See the git history for details.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

MODULE = github.com/jimyag/oci2vm

TAG = $(shell git diff --quiet && git describe --tags --exact-match 2> /dev/null)
VERSION_SUFFIX = $(shell git diff --quiet || echo "-dev")
VERSION = $(shell git describe --tags --exact-match 2> /dev/null || echo "`git describe --tags $$(git rev-list --tags --max-count=1) 2> /dev/null || echo v0.0.0`-`git rev-parse --short HEAD`")$(VERSION_SUFFIX)
show-version:
	@echo $(VERSION)

GORELEASER_VERSION := v2.18.3
GORELEASER_URL := https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)/goreleaser_Linux_x86_64.tar.gz

BIN := $(PWD)/bin
export PATH := $(BIN):$(PATH)

CLI_REFERENCE_PATH := docs/content/reference

bin:
	@mkdir -p $(BIN)
	@curl -sL $(GORELEASER_URL) | tar -C $(BIN) -xz goreleaser

clean-bin:
	@rm -rf $(BIN)

DOCKER_IMAGE := ghcr.io/jimyag/oci2vm

docker: docker-build docker-push

docker-push:
	@docker image push $(DOCKER_IMAGE):$(VERSION)
ifneq ($(TAG),)
	@docker image push $(DOCKER_IMAGE):latest
endif

docker-build:
	@docker image build -t $(DOCKER_IMAGE):$(VERSION) .
ifneq ($(TAG),)
	@docker image tag $(DOCKER_IMAGE):$(TAG) $(DOCKER_IMAGE):latest
endif

docker-run:
	@docker run --rm -i -t \
		--privileged \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD):/build \
		-w /build \
		$(DOCKER_IMAGE) bash

.PHONY: tests
tests:
	@go generate ./...
	@go list .| xargs go test -exec sudo -count=1 -timeout 60m -v -skip TestConfig

.PHONY: test-templates
test-templates:
	@go generate ./...
	@go test -exec sudo -count=1 -timeout 60m -v -run TestConfig/$(IMAGE)

e2e: docker-build .build
	@go test -v -exec sudo -count=1 -timeout 60m -ldflags "-X '$(MODULE).Version=$(VERSION)' -X '$(MODULE).BuildDate=$(shell date)'" ./e2e -args -images $(E2E_IMAGES)

docs-up-to-date:
	@$(MAKE) cli-docs
	@git diff --quiet -- docs ':(exclude)docs/content/reference/oci2vm_run_qemu.md' || (git --no-pager diff -- docs ':(exclude)docs/content/reference/oci2vm_run_qemu.md'; echo "Please regenerate the documentation with 'make cli-docs'"; exit 1)

.PHONY: lint
lint:
	@go generate ./...
	@golangci-lint run ./...

build-dev: docker-build .build

install: docker-build
	@go generate ./...
	@go install -ldflags "-s -w -X '$(MODULE).Version=$(VERSION)' -X '$(MODULE).BuildDate=$(shell date)'" ./cmd/oci2vm

.build:
	@go generate ./...
	@CGO_ENABLED=0 go build -o oci2vm -ldflags "-s -w -X '$(MODULE).Version=$(VERSION)' -X '$(MODULE).BuildDate=$(shell date)'" ./cmd/oci2vm

.PHONY: build-snapshot
build-snapshot: bin
	@VERSION=$(VERSION) IMAGE=$(DOCKER_IMAGE) goreleaser build --snapshot --clean --parallelism 8

.PHONY: release-snapshot
release-snapshot: bin
	@VERSION=$(VERSION) IMAGE=$(DOCKER_IMAGE) goreleaser release --snapshot --clean --skip=announce,publish --parallelism 8

.PHONY: build
build: bin
	@VERSION=$(VERSION) IMAGE=$(DOCKER_IMAGE) goreleaser build --clean --parallelism 8

.PHONY: release
release: bin
	@VERSION=$(VERSION) IMAGE=$(DOCKER_IMAGE) goreleaser release --clean --parallelism 8

.PHONY: completions
completions: .build
	@rm -rf completions
	@mkdir -p completions
	@for shell in bash zsh fish powershell; do \
		./oci2vm completion $$shell > completions/oci2vm.$$shell; \
	done

.PHONY: examples
examples: build-dev
	@mkdir -p examples/build
	@for f in $$(find examples -maxdepth 1 -type f -name '*Dockerfile'); do \
  		echo "Building $$f"; \
  		./oci2vm build -o examples/build/$$(basename $$f|cut -d'.' -f1).qcow2 -p root -f $$f examples --force; \
	  done
	@echo "Building examples/app"
	@./oci2vm build -o examples/build/app.qcow2 examples/app --force

cli-docs: .build
	@rm -rf $(CLI_REFERENCE_PATH)
	@./oci2vm docs $(CLI_REFERENCE_PATH)

serve-docs:
	@docker run --rm -i -t --user=$(UID) -p 8000:8000 -v $(PWD):/docs squidfunk/mkdocs-material serve -f /docs/docs/mkdocs.yml -a 0.0.0.0:8000

.PHONY: build-docs
build-docs: clean-docs cli-docs
	@docker run --rm -v $(PWD):/docs --user $$(id -u):$$(id -u) squidfunk/mkdocs-material build -f /docs/docs/mkdocs.yml -d build

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

GITHUB_PAGES_BRANCH := gh-pages

deploy-docs:
	@git branch -D gh-pages &> /dev/null || true
	@git checkout -b $(GITHUB_PAGES_BRANCH)
	@rm .gitignore && mv docs docs-src && mv docs-src/build docs && rm -rf docs-src
	@git add . && git commit -m "build docs" && git push origin --force $(GITHUB_PAGES_BRANCH)
	@git checkout $(GIT_BRANCH)

docs: cli-docs build-docs deploy-docs

clean-docs:
	@rm -rf docs/build
