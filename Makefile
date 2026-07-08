APP_NAME    := qmezzotone
MAIN_PKG    := .
BUILD_DIR   := build
GO          := go
GOFLAGS     := -trimpath
LDFLAGS     := -s -w

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: all build clean test $(PLATFORMS)

all: build

build: $(PLATFORMS)

$(PLATFORMS):
	@$(eval GOOS = $(word 1,$(subst /, ,$@)))
	@$(eval GOARCH = $(word 2,$(subst /, ,$@)))
	@$(eval EXT = $(if $(filter windows,$(GOOS)),.exe,))
	@mkdir -p $(BUILD_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME)-$(GOOS)-$(GOARCH)$(EXT) $(MAIN_PKG)

clean:
	rm -rf $(BUILD_DIR)

test:
	$(GO) test ./...
