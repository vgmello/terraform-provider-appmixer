# Provider name as used in Terraform configurations and rendered into the
# generated doc pages. Passing both keeps page titles as "appmixer …" — a bare
# `tfplugindocs generate` derives them from the repository directory instead and
# rewrites every page.
PROVIDER_NAME := appmixer

.PHONY: build test e2e docs docs-check fmt

build:
	go build ./...

test:
	go test ./...

e2e:
	go test -tags e2e -v ./e2e/...

# Regenerates docs/ from the provider schemas, examples/ and templates/.
# Hand-written pages live in templates/ — anything only in docs/ is deleted.
docs:
	go tool tfplugindocs generate \
		--provider-name $(PROVIDER_NAME) \
		--rendered-provider-name $(PROVIDER_NAME)

# Fails when the committed docs/ differ from what the current schemas produce.
# Checks porcelain status rather than `git diff` so an entirely new, untracked
# page (a resource added without docs) is caught too.
docs-check: docs
	@if [ -n "$$(git status --porcelain -- docs/)" ]; then \
		git status --short -- docs/; \
		echo "docs/ is out of date — run 'make docs' and commit the result"; \
		exit 1; \
	fi

fmt:
	gofmt -w .
	terraform fmt -recursive examples/
