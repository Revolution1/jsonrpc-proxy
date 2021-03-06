VERSION:=$(shell grep -oP '(?<=var version = ")[^"]*' version.go)
COMMIT:=$(shell git describe --dirty --always)
BRANCH:=$(shell git rev-parse --abbrev-ref HEAD)
TAG:=$(shell git describe --exact-match HEAD --tags 2>/dev/null)
DATE=$(shell date +%s)
BUILD_INFO=$(shell go version)

DIST="./dist"
BIN_NAME="jsonrpc-proxy"
MODULE_NAME="github.com/revolution1/jsonrpc-proxy"
#IMAGE_REGISTRY="ghcr.io/"

ifdef TAG
	IMAGE="${IMAGE_REGISTRY}revolution1/jsonrpc-proxy:${TAG}"
else
	IMAGE="${IMAGE_REGISTRY}revolution1/jsonrpc-proxy:${VERSION}-${COMMIT}"
endif

BUILD_FLAGS=-v -ldflags '-s -w \
	-X "main.commit=${COMMIT}" \
	-X "main.tag=${TAG}" \
	-X "main.branch=${BRANCH}" \
	-X "main.date=${DATE}" \
	-X "main.buildInfo=${BUILD_INFO}"'

#DOCKER_BUILD_ARG=\
#	--build-arg VERSION="${VERSION}" \
#	--build-arg COMMIT="${COMMIT}" \
#	--build-arg BRANCH="${BRANCH}" \
#	--build-arg TAG="${TAG}" \
#	--build-arg DATE="${DATE}" \
#	--build-arg BUILD_INFO="${BUILD_INFO}"

info:
	@echo 'Version: ' ${VERSION}
	@echo 'Branch:  ' ${BRANCH}
	@echo 'Commit:  ' ${COMMIT}
	@echo 'Dist Dir:' ${DIST}
	@echo 'Tag  :   ' ${TAG}
	@echo 'Image:   ' ${IMAGE}
	@echo
	@echo 'Use "make release" to build release binaries'
	@echo 'Use "make local" to build binary for local environment'

clean:
	rm -rf ./dist

test:
	go test -v ./...

local:
	mkdir -p ${DIST}
	GO111MODULE="on" go build ${BUILD_FLAGS} -o ${DIST}/${BIN_NAME} ${MODULE_NAME}

benchmark: local
	GO111MODULE="on" go run ${MODULE_NAME}/test/benchmark -p ${DIST}/${BIN_NAME}

linux-amd64:
	mkdir -p ${DIST}
	GO111MODULE="on" GOOS=linux GOARCH=amd64 go build ${BUILD_FLAGS} -o ${DIST}/${BIN_NAME}-Linux-amd64 ${MODULE_NAME}

darwin-amd64:
	mkdir -p ${DIST}
	GO111MODULE="on" GOOS=darwin GOARCH=amd64 go build ${BUILD_FLAGS} -o ${DIST}/${BIN_NAME}-Darwin-amd64 ${MODULE_NAME}


image:
	#docker build -t ${IMAGE} . ${DOCKER_BUILD_ARG}
	docker build -t ${IMAGE} .

push-image:
	docker push ${IMAGE}

release: clean linux-amd64 darwin-amd64
	#rm -f ${DIST}/sha256sums.txt
	cd ${DIST} && sha256sum ./* > sha256sums.txt

tag-release:
	git tag v${VERSION}
