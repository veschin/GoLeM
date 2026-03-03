.PHONY: build test lint clean install release

VERSION ?= patch
BINARY := glm
CMD := ./cmd/glm

build:
	go build -o $(BINARY) $(CMD)

test:
	go test -v ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)

install:
	go install $(CMD)

release:
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "error: uncommitted changes"; \
		exit 1; \
	fi
	@CURRENT_TAG=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	CURRENT=$$(echo $$CURRENT_TAG | sed 's/^v//'); \
	MAJOR=$$(echo $$CURRENT | cut -d. -f1); \
	MINOR=$$(echo $$CURRENT | cut -d. -f2); \
	PATCH=$$(echo $$CURRENT | cut -d. -f3); \
	case $(VERSION) in \
		major) NEW_MAJOR=$$((MAJOR + 1)); NEW_MINOR=0; NEW_PATCH=0 ;; \
		minor) NEW_MAJOR=$$MAJOR; NEW_MINOR=$$((MINOR + 1)); NEW_PATCH=0 ;; \
		patch) NEW_MAJOR=$$MAJOR; NEW_MINOR=$$MINOR; NEW_PATCH=$$((PATCH + 1)) ;; \
		*) echo "error: VERSION must be major, minor, or patch"; exit 1 ;; \
	esac; \
	NEW_TAG="v$$NEW_MAJOR.$$NEW_MINOR.$$NEW_PATCH"; \
	echo "Creating tag $$NEW_TAG"; \
	git tag $$NEW_TAG && git push origin $$NEW_TAG
