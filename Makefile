DOCKER_REGISTRY ?= jspdown
TEST_OPTS :=

# Builtin targets:
##################

.PHONY: default
default: lint test build

.PHONY: all
all: clean lint test build

# Common targets:
#################

GO_SOURCES := $(shell find . -name '*.go')
BUILD_FLAGS := -trimpath -ldflags "-w -s"

.PHONY: build
build: ./dist/traefik-playground

./dist/traefik-playground: app-build ./dist ./go.mod $(GO_SOURCES)
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -o $@ ./cmd

./dist:
	mkdir -p dist

.PHONY: build-image-%
build-image-%:
	docker build $(DOCKER_ARGS) -t $(DOCKER_REGISTRY)/traefik-playground:$* -f ./deployments/Dockerfile .

.PHONY: dev
dev: build-image-dev
	RELEASE_VERSION=dev docker compose -f ./deployments/docker-compose.yaml up -d

.PHONY: lint
lint: app-lint
	golangci-lint run

.PHONY: test
test: app-test
	go test -v -cover ./... $(TEST_OPTS)

.PHONY: clean
clean: app-clean
	rm -rf cover.out
	rm -rf dist

# App target:
#############

.PHONY: app-build
app-build:
	$(MAKE) -C app build

.PHONY: app-lint
app-lint:
	$(MAKE) -C app lint

.PHONY: app-test
app-test:
	$(MAKE) -C app test

.PHONY: app-clean
app-clean:
	$(MAKE) -C app clean

# Tool targets:
###############

.PHONY: generate-json-schemas
generate-json-schemas:
	$(MAKE) -C app generate-json-schemas

.PHONY: new-migration
new-migration:
	@read -p "Name : " name; docker run -u $(shell id -u):$(shell id -g) -v ./db/migrations:/migrations migrate/migrate create -ext .sql -dir /migrations/ $${name}

