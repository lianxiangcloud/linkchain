
DEFAULT_GOOS=$(shell go env | grep -o 'GOOS=".*"' | sed -E 's/GOOS="(.*)"/\1/g')
DEFAULT_GOARCH=$(shell go env | grep -o 'GOARCH=".*"' | sed -E 's/GOARCH="(.*)"/\1/g')
PACKAGES=$(shell go list ./... | grep -v '/vendor/')
GIT_VER=$(shell git rev-parse --short=8 HEAD)

#BUILD_TAGS= -tags ''
BUILD_FLAGS = -ldflags "-X github.com/lianxiangcloud/linkchain/version.GitCommit=$(GIT_VER)"

all: build

########################################
### Build
define fbuild
	CGO_ENABLED=1 GOOS=$(1) GOARCH=$(2) go build $(3) $(BUILD_FLAGS) $(BUILD_TAGS) -o bin/lkchain ./cmd/lkchain
	CGO_ENABLED=1 GOOS=$(1) GOARCH=$(2) go build $(3) $(BUILD_FLAGS) $(BUILD_TAGS) -o bin/wallet ./wallet/cmd
endef

bench:
	CGO_ENABLED=1 go build $(BUILD_FLAGS) $(BUILD_TAGS) -o bin/bench ./test/bench

build:
	$(call fbuild,$(DEFAULT_GOOS),$(DEFAULT_GOARCH))
	
build-linux:
	$(call fbuild,linux,amd64)

build-darwin:
	$(call fbuild,darwin,amd64)

build_race:
	$(call fbuild,$(DEFAULT_GOOS),$(DEFAULT_GOARCH),-race)
	
install:
	CGO_ENABLED=1 GOOS=$(DEFAULT_GOOS) GOARCH=$(DEFAULT_GOARCH) go install $(BUILD_FLAGS) $(BUILD_TAGS) ./cmd/lkchain

########################################
### Testing

## required to be run first by most tests
build_docker_test_image:
	docker build -t tester -f ./test/docker/Dockerfile .

### coverage, app, persistence, and libs tests
test_cover:
	# run the go unit tests with coverage
	bash test/test_cover.sh

test_release:
	@go test -tags release $(PACKAGES)

test100:
	@for i in {1..100}; do make test; done

### go tests
test:
	@echo "--> Running go test"
	@go test $(PACKAGES)

test_race:
	@echo "--> Running go test --race"
	@go test -v -race $(PACKAGES)

# To avoid unintended conflicts with file names, always add to .PHONY
# unless there is a reason not to.
# https://www.gnu.org/software/make/manual/html_node/Phony-Targets.html
.PHONY: check build build_race install test_cover test test_race test_release test100 localnet-start localnet-stop build-docker
