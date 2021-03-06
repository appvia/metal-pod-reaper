NAME=mpodr
AUTHOR=appvia
AUTHOR_EMAIL=lewis.marshall@appvia.io
BINARY ?= ${NAME}
CONTAINER ?= quay.io/${AUTHOR}/${NAME}
ROOT_DIR=${PWD}
HARDWARE=$(shell uname -m)
GIT_VERSION=$(shell git describe --always --tags --dirty)
GIT_SHA=$(shell git rev-parse HEAD)
GOVERSION=1.12
export GO111MODULE=on
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%I:%M:%S%p')
VERSION ?= ${GIT_VERSION}
DEPS=$(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
PACKAGES=$(shell go list ./...)
VERSION_PKG=$(shell go list ./pkg/version)
LDFLAGS ?= '-extldflags "-static"' -X ${VERSION_PKG}.gitVersion=${GIT_VERSION} -X ${VERSION_PKG}.gitSha=${GIT_SHA}
VETARGS ?= -asmdecl -atomic -bool -buildtags -copylocks -methods -nilfunc -printf -rangeloops -unsafeptr
PLATFORM=linux
ARCH=amd64

.PHONY: test authors changelog build release lint cover vet

default: build

golang:
	@echo "--> Go Version"
	@go version

build:
	@echo "--> Compiling the project"
	mkdir -p bin
	go build -ldflags "${LDFLAGS}" -o bin/${NAME} *.go

build_caps: build
	sudo setcap cap_net_raw=+ep bin/${NAME}

docker_build:
	@echo "--> Creating a container"
	docker build . -t ${CONTAINER}:${VERSION}

docker_push:
	@echo "--> Pushing container"
	docker push ${CONTAINER}:${VERSION}

release: clean
	@echo "--> Compiling all the static binaries"
	mkdir -p bin
	 CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o ./bin/${NAME}_${PLATFORM}_${ARCH} *.go

clean:
	rm -rf ./bin 2>/dev/null

authors:
	@echo "--> Updating the AUTHORS"
	git log --format='%aN <%aE>' | sort -u > AUTHORS

vet:
	@echo "--> Running go vet $(VETARGS) ."
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi
	@go vet $(VETARGS) $(PACKAGES)

lint:
	@echo "--> Running golint"
	@which golint 2>/dev/null ; if [ $$? -eq 1 ]; then \
		go get -u golang.org/x/lint/golint; \
	fi
	@golint .

gofmt:
	@echo "--> Running gofmt check"
	@gofmt -s -l $(PACKAGES) | grep -q \.go ; if [ $$? -eq 0 ]; then \
      echo "we have unformatted files - run 'make applygofmt' to apply"; \
			gofmt -s -d -l ${PACKAGES}; \
      exit 1; \
    fi

applygofmt:
	@echo "--> Running gofmt apply"
	@gofmt -s -l -w $(PACKAGES)

bench:
	@echo "--> Running go bench"
	@go test -v -bench=.

coverage:
	@echo "--> Running go coverage"
	@go test -coverprofile cover.out
	@go tool cover -html=cover.out -o cover.html

cover:
	@echo "--> Running go cover"
	@go test -cover $(PACKAGES)

test:
	@echo "--> Running the tests"
	@go test -v
	@$(MAKE) cover

src:
	@echo "--> Running the src checks"
	@$(MAKE) vet
	@$(MAKE) lint
	@$(MAKE) gofmt

changelog: release
	git log $(shell git tag | tail -n1)..HEAD --no-merges --format=%B > changelog
