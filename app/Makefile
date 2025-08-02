JS_SOURCES := $(wildcard js/*.js)
ASSET_SOURCES := $(wildcard assets/*)
CONFIG_FILES := vite.config.js package.json

STATIC_ASSETS := $(patsubst assets/%,dist/%,$(ASSET_SOURCES))

dist/index.js: .pnp.cjs $(JS_SOURCES) $(CONFIG_FILES)
	yarn build

$(STATIC_ASSETS): dist/%: assets/%
	@mkdir -p dist
	cp $< $@

.PHONY: build-js
build-js: dist/index.js

.PHONY: build-assets
build-assets: $(STATIC_ASSETS)

.PHONY: build
build: build-js build-assets

.PHONY: lint
lint: build
	yarn lint

.PHONY: test
test: build
	yarn test

yarn.lock: package.json
	yarn install
	touch $@

.pnp.cjs: yarn.lock
	yarn install

.PHONY: clean
clean:
	rm -rf dist .pnp.cjs

.PHONY: install
install: yarn.lock

.PHONY: generate-json-schemas
generate-json-schemas:
	go run ../tools/json-schema-gen | jq > js/traefik-v3.schema.json
