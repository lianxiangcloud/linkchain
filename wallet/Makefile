BUILD_FLAGS = -ldflags "-X main.GitCommit=`git rev-parse HEAD` "
#-X main.GitBranch=`git branch --contains HEAD`

DEFAULT_GOOS=$(shell go env | grep -o 'GOOS=".*"' | sed -E 's/GOOS="(.*)"/\1/g')
DEFAULT_GOARCH=$(shell go env | grep -o 'GOARCH=".*"' | sed -E 's/GOARCH="(.*)"/\1/g')

all:
	GOOS=$(DEFAULT_GOOS) GOARCH=$(DEFAULT_GOARCH) go build $(BUILD_FLAGS) -o bin/wallet ./cmd/

clean:
	rm -vf bin/*
	rm -vf *.log
