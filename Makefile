PACKAGE_NAME          := shopware-cli
GOLANG_CROSS_VERSION  ?= latest
PUBLISH               ?= 0
VSCODE_OS             ?= $(OS)

.PHONY: release-dry-run
release-dry-run:
	@docker run \
		--rm \
		-e CGO_ENABLED=1 \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		--clean --skip=validate --skip=publish --snapshot

.PHONY: release
release:
	docker run \
		--rm \
		-e CGO_ENABLED=1 \
		-e GITHUB_TOKEN \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --clean

.PHONY: release-build-extension
release-build-extension:
	mkdir -p dist
	$(eval tmpDir := $(shell mktemp -d))
	@curl -q -L -o "${tmpDir}/shopware-lsp.zip" https://github.com/shopwareLabs/shopware-lsp/releases/download/${VERSION}/shopware-lsp_${VERSION}_$(OS)_$(ARCH).zip
	@unzip -q "${tmpDir}/shopware-lsp.zip" -d "${tmpDir}"
	@cp "${tmpDir}/shopware-lsp" ./vscode-extension/shopware-lsp
	@rm -rf "${tmpDir}"
	$(eval RELEASE_ARCH := $(if $(filter amd64,$(ARCH)),x64,$(ARCH)))
	npm install --prefix ./vscode-extension
	cd vscode-extension && jq '.version = "${VERSION}"' package.json > package.json.tmp && mv package.json.tmp package.json
	cd vscode-extension && npx @vscode/vsce package --target ${VSCODE_OS}-${RELEASE_ARCH} --pre-release -o ../dist/shopware-lsp-${VERSION}-${VSCODE_OS}-${RELEASE_ARCH}.vsix
	rm -rf ./vscode-extension/shopware-lsp
	gh release upload ${VERSION} ./dist/shopware-lsp-${VERSION}-${VSCODE_OS}-${RELEASE_ARCH}.vsix
	@if [ "${PUBLISH}" = "1" ]; then \
		npx @vscode/vsce publish --packagePath ./dist/shopware-lsp-${VERSION}-${VSCODE_OS}-${RELEASE_ARCH}.vsix; \
	else \
		echo "Skipping VSCode extension publish. Set PUBLISH=1 to publish."; \
	fi
