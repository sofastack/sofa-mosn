SHELL = /bin/bash

TARGET          = mosnd
TARGET_SIDECAR  = mosn
CONFIG_FILE     = mosn_config.json
PROJECT_NAME    = mosn.io/mosn

ISTIO_VERSION   = 1.5.2

SCRIPT_DIR      = $(shell pwd)/etc/script

MAJOR_VERSION   = $(shell cat VERSION)
GIT_VERSION     = $(shell git log -1 --pretty=format:%h)
GIT_NOTES       = $(shell git log -1 --oneline)

BUILD_IMAGE     = godep-builder

WASM_IMAGE      = mosn-wasm

IMAGE_NAME      ?= mosn
REPOSITORY      = mosnio/${IMAGE_NAME}

RPM_BUILD_IMAGE = afenp-rpm-builder
RPM_VERSION     = $(shell cat VERSION | tr -d '-')
RPM_TAR_NAME    = afe-${TARGET}
RPM_SRC_DIR     = ${RPM_TAR_NAME}-${RPM_VERSION}
RPM_TAR_FILE    = ${RPM_SRC_DIR}.tar.gz

TAGS			= ${tags}
TAGS_OPT 		=

# support build custom tags
ifneq ($(TAGS),)
TAGS_OPT 		= -tags ${TAGS}
endif


ut-local:
	GO111MODULE=off go test -gcflags=-l -v `go list ./pkg/... | grep -v pkg/mtls/crypto/tls | grep -v pkg/networkextention`

unit-test:
	docker build --rm -t ${BUILD_IMAGE} build/contrib/builder/binary
	docker run --rm -v $(shell go env GOPATH):/go -v $(shell pwd):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} ${BUILD_IMAGE} make ut-local

coverage-local:
	sh ${SCRIPT_DIR}/report.sh

coverage:
	docker build --rm -t ${BUILD_IMAGE} build/contrib/builder/binary
	docker run --rm -v $(shell go env GOPATH):/go -v $(shell pwd):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} ${BUILD_IMAGE} make coverage-local

integrate-local:
	GO111MODULE=off go test -p 1 -v ./test/integrate/...

integrate-local-netpoll:
	GO111MODULE=off NETPOLL=on go test -p 1 -v ./test/integrate/...

integrate:
	docker build --rm -t ${BUILD_IMAGE} build/contrib/builder/binary
	docker run --rm -v $(shell go env GOPATH):/go -v $(shell pwd):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} ${BUILD_IMAGE} make integrate-local


integrate-netpoll:
	docker build --rm -t ${BUILD_IMAGE} build/contrib/builder/binary
	docker run --rm -v $(shell go env GOPATH):/go -v $(shell pwd):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} ${BUILD_IMAGE} make integrate-local-netpoll

integrate-framework:
	@cd ./test/cases && bash run_all.sh

integrate-new:
	docker build --rm -t ${BUILD_IMAGE} build/contrib/builder/binary
	docker run --rm -v $(shell go env GOPATH):/go -v $(shell pwd):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} ${BUILD_IMAGE} make integrate-framework

build:
	docker build --rm -t ${BUILD_IMAGE} build/contrib/builder/binary
	docker run --rm -v $(shell pwd):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} ${BUILD_IMAGE} make build-local

build-host:
	docker build --rm -t ${BUILD_IMAGE} build/contrib/builder/binary
	docker run --net=host --rm -v $(shell pwd):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} ${BUILD_IMAGE} make build-local

build-wasm-image:
	docker build --rm -t ${WASM_IMAGE}:${MAJOR_VERSION} -f build/contrib/builder/wasm/Dockerfile .

binary: build

binary-host: build-host

build-local:
	@rm -rf build/bundles/${MAJOR_VERSION}/binary
	GO111MODULE=off CGO_ENABLED=1 go build ${TAGS_OPT} \
		-ldflags "-B 0x$(shell head -c20 /dev/urandom|od -An -tx1|tr -d ' \n') -X main.Version=${MAJOR_VERSION}(${GIT_VERSION}) -X ${PROJECT_NAME}/pkg/types.IstioVersion=${ISTIO_VERSION}" \
		-v -o ${TARGET} \
		${PROJECT_NAME}/cmd/mosn/main
	mkdir -p build/bundles/${MAJOR_VERSION}/binary
	mv ${TARGET} build/bundles/${MAJOR_VERSION}/binary
	@cd build/bundles/${MAJOR_VERSION}/binary && $(shell which md5sum) -b ${TARGET} | cut -d' ' -f1  > ${TARGET}.md5
	cp configs/${CONFIG_FILE} build/bundles/${MAJOR_VERSION}/binary
	cp build/bundles/${MAJOR_VERSION}/binary/${TARGET}  build/bundles/${MAJOR_VERSION}/binary/${TARGET_SIDECAR}

build-linux32:
	@rm -rf build/bundles/${MAJOR_VERSION}/binary
	GO111MODULE=off CGO_ENABLED=1 env GOOS=linux GOARCH=386 go build ${TAGS_OPT} \
		-ldflags "-B 0x$(shell head -c20 /dev/urandom|od -An -tx1|tr -d ' \n') -X main.Version=${MAJOR_VERSION}(${GIT_VERSION}) -X ${PROJECT_NAME}/pkg/types.IstioVersion=${ISTIO_VERSION}" \
		-v -o ${TARGET} \
		${PROJECT_NAME}/cmd/mosn/main
	mkdir -p build/bundles/${MAJOR_VERSION}/binary
	mv ${TARGET} build/bundles/${MAJOR_VERSION}/binary
	@cd build/bundles/${MAJOR_VERSION}/binary && $(shell which md5sum) -b ${TARGET} | cut -d' ' -f1  > ${TARGET}.md5
	cp configs/${CONFIG_FILE} build/bundles/${MAJOR_VERSION}/binary
	cp build/bundles/${MAJOR_VERSION}/binary/${TARGET}  build/bundles/${MAJOR_VERSION}/binary/${TARGET_SIDECAR}

build-linux64:
	@rm -rf build/bundles/${MAJOR_VERSION}/binary
	GO111MODULE=off CGO_ENABLED=1 env GOOS=linux GOARCH=amd64 go build ${TAGS_OPT} \
		-ldflags "-B 0x$(shell head -c20 /dev/urandom|od -An -tx1|tr -d ' \n') -X main.Version=${MAJOR_VERSION}(${GIT_VERSION}) -X ${PROJECT_NAME}/pkg/types.IstioVersion=${ISTIO_VERSION}" \
		-v -o ${TARGET} \
		${PROJECT_NAME}/cmd/mosn/main
	mkdir -p build/bundles/${MAJOR_VERSION}/binary
	mv ${TARGET} build/bundles/${MAJOR_VERSION}/binary
	@cd build/bundles/${MAJOR_VERSION}/binary && $(shell which md5sum) -b ${TARGET} | cut -d' ' -f1  > ${TARGET}.md5
	cp configs/${CONFIG_FILE} build/bundles/${MAJOR_VERSION}/binary
	cp build/bundles/${MAJOR_VERSION}/binary/${TARGET}  build/bundles/${MAJOR_VERSION}/binary/${TARGET_SIDECAR}


image:
	@rm -rf IMAGEBUILD
	cp -r build/contrib/builder/image IMAGEBUILD && cp build/bundles/${MAJOR_VERSION}/binary/${TARGET} IMAGEBUILD && cp -r configs IMAGEBUILD && cp -r etc IMAGEBUILD
	docker build --no-cache --rm -t ${IMAGE_NAME}:${MAJOR_VERSION}-${GIT_VERSION} IMAGEBUILD
	docker tag ${IMAGE_NAME}:${MAJOR_VERSION}-${GIT_VERSION} ${REPOSITORY}:${MAJOR_VERSION}-${GIT_VERSION}
	rm -rf IMAGEBUILD

rpm:
	@sleep 1  # sometimes device-mapper complains for a relax
	docker build --rm -t ${RPM_BUILD_IMAGE} build/contrib/builder/rpm
	docker run --rm -w /opt/${TARGET}     \
		-v $(shell pwd):/opt/${TARGET}    \
		-e RPM_GIT_VERSION=${GIT_VERSION} \
		-e "RPM_GIT_NOTES=${GIT_NOTES}"   \
	   	${RPM_BUILD_IMAGE} make rpm-build-local

rpm-build-local:
	@rm -rf build/bundles/${MAJOR_VERSION}/rpm
	mkdir -p build/bundles/${MAJOR_VERSION}/rpm/${RPM_SRC_DIR}
	cp -r build/bundles/${MAJOR_VERSION}/binary/${TARGET} build/bundles/${MAJOR_VERSION}/rpm/${RPM_SRC_DIR}
	cp -r build/bundles/${MAJOR_VERSION}/binary/${CONFIG_FILE} build/bundles/${MAJOR_VERSION}/rpm/${RPM_SRC_DIR}
	cp build/contrib/builder/rpm/${TARGET}.spec build/bundles/${MAJOR_VERSION}/rpm
	cp build/contrib/builder/rpm/${TARGET}.service build/bundles/${MAJOR_VERSION}/rpm/${RPM_SRC_DIR}
	cp build/contrib/builder/rpm/${TARGET}.logrotate build/bundles/${MAJOR_VERSION}/rpm/${RPM_SRC_DIR}
	cd build/bundles/${MAJOR_VERSION}/rpm && tar zcvf ${RPM_TAR_FILE} ${RPM_SRC_DIR}
	mv build/bundles/${MAJOR_VERSION}/rpm/${RPM_TAR_FILE} ~/rpmbuild/SOURCES
	chown -R root:root ~/rpmbuild/SOURCES/${RPM_TAR_FILE}
	rpmbuild -bb --clean build/contrib/builder/rpm/${TARGET}.spec             	\
			--define "AFENP_NAME       ${RPM_TAR_NAME}" 					\
			--define "AFENP_VERSION    ${RPM_VERSION}"       		    	\
			--define "AFENP_RELEASE    ${RPM_GIT_VERSION}"     				\
			--define "AFENP_GIT_NOTES '${RPM_GIT_NOTES}'"
	cp ~/rpmbuild/RPMS/x86_64/*.rpm build/bundles/${MAJOR_VERSION}/rpm
	rm -rf build/bundles/${MAJOR_VERSION}/rpm/${RPM_SRC_DIR} build/bundles/${MAJOR_VERSION}/rpm/${TARGET}.spec

shell:
	docker build --rm -t ${BUILD_IMAGE} build/contrib/builder/binary
	docker run --rm -ti -v $(shell go env GOPATH):/go -v $(shell pwd):/go/src/${PROJECT_NAME} -w /go/src/${PROJECT_NAME} ${BUILD_IMAGE} /bin/bash

sidecar:
	docker build --no-cache -t ${REPOSITORY}:${MAJOR_VERSION}-${GIT_VERSION}-$(shell date '+%Y%m%d%H%M') -f build/contrib/builder/sidecar/Dockerfile .

.PHONY: unit-test build image rpm upload shell sidecar
