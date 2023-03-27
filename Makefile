# Copyright 2023 TriggerMesh Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

KREPO      = brokers
KREPO_DESC = TriggerMesh Brokers

BASE_DIR          ?= $(CURDIR)
OUTPUT_DIR        ?= $(BASE_DIR)/_output

BIN_OUTPUT_DIR    ?= $(OUTPUT_DIR)
DIST_DIR          ?= $(OUTPUT_DIR)

# Dynamically generate the list of commands based on the directory name cited in the cmd directory
BROKERS           := $(notdir $(wildcard cmd/*))

IMAGE_REPO        ?= gcr.io/triggermesh
IMAGE_TAG         ?= $(shell git rev-parse HEAD)

# Go build variables
GO                ?= go
GOFMT             ?= gofmt

GOPKGS             = ./cmd/... ./pkg/backend/... ./pkg/broker/... ./pkg/common/... ./pkg/config/... ./pkg/ingest/... ./pkg/subscriptions/...

LDFLAGS            = -w -s
LDFLAGS_STATIC     = $(LDFLAGS) -extldflags=-static

TAG_REGEX         := ^v([0-9]{1,}\.){2}[0-9]{1,}$

.PHONY: help all build release fmt fmt-test images clean

all: build

help: ## Display this help
	@awk 'BEGIN {FS = ":.*?## "; printf "\n$(KREPO_DESC)\n\nUsage:\n  make \033[36m<cmd>\033[0m\n"} /^[a-zA-Z0-9._-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: $(BROKERS)  ## Build all artifacts

$(BROKERS): ## Build artifact
	$(GO) build -ldflags "$(LDFLAGS_STATIC)" -o $(BIN_OUTPUT_DIR)/$@ ./cmd/$@

release: ## Publish container images
	$(KO) build $(KOFLAGS) -B -t $(IMAGE_TAG) --tag-only ./cmd/*
ifeq ($(shell echo ${IMAGE_TAG} | egrep "${TAG_REGEX}"),${IMAGE_TAG})
	$(KO) build $(KOFLAGS) -B -t latest ./cmd/*
endif

fmt: ## Format source files
	$(GOFMT) -s -w $(shell $(GO) list -f '{{$$d := .Dir}}{{range .GoFiles}}{{$$d}}/{{.}} {{end}} {{$$d := .Dir}}{{range .TestGoFiles}}{{$$d}}/{{.}} {{end}}' $(GOPKGS))

fmt-test: ## Check source formatting
	@test -z $(shell $(GOFMT) -l $(shell $(GO) list -f '{{$$d := .Dir}}{{range .GoFiles}}{{$$d}}/{{.}} {{end}} {{$$d := .Dir}}{{range .TestGoFiles}}{{$$d}}/{{.}} {{end}}' $(GOPKGS)))

IMAGES = $(foreach runtime,$(BROKERS),$(runtime).image)
images: $(IMAGES) ## Build container images
$(IMAGES): %.image:
	docker build -t $(IMAGE_REPO)/$*:${IMAGE_TAG} -f cmd/$*/Dockerfile .

clean: ## Clean build artifacts
	@for bin in $(BROKERS) ; do \
		$(RM) -v $(BIN_OUTPUT_DIR)/$$bin; \
	done
