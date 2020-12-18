DB_URL:=oracle://system:oracle@$(shell ip route get 8.8.8.8 | head -1 | gawk '{ print $$7 }'):1521/xe

# TESTS
# ==============================================================================
PHONY+=test/unit
test/unit:
	GOPROXY=$(shell go env GOPROXY) \
		go test -v -race -coverprofile=coverage.txt -covermode=atomic -timeout 900s -count=1 ./...

PHONY+=test/coverage
test/coverage: test/unit
	go tool cover -html=coverage.txt

# CONTAINER STEPS - TESTS
# ==============================================================================
PHONY+=container-run/test/unit
container-run/test/unit:
	$(MAKE) container-run COMMAND=${@:container-run/%=%}

PHONY+=container-run/test/coverage
container-run/test/coverage:
	$(MAKE) container-run COMMAND=${@:container-run/%=%}

# GENERAL CONTAINER COMMAND
# ==============================================================================
PHONY+=container-run
container-run:
	docker run \
		--env DB_URL=${DB_URL} \
		--env GOPROXY=direct \
		--env DPI_DEBUG_LEVEL=30 \
		--volume $(shell pwd):/workspace \
		--volume /var/run/docker.sock:/var/run/docker.sock \
		--net=host \
		--workdir /workspace \
		--rm \
			docker.io/volkerraschek/build-image@sha256:cf3f61b821dacc5c938399cae7101a5122d153a837d0f15f72803a5cb7fc3640 \
				make ${COMMAND}

start-oracle-xe:
	docker run -e DBCA_TOTAL_MEMORY=4096 --name oracle-db --detach --publish 1521:1521/tcp quay.io/maksymbilenko/oracle-12c

stop-oracle-xe:
	docker rm --force oracle-db

# PHONY
# ==============================================================================
# Declare the contents of the PHONY variable as phony. We keep that information
# in a variable so we can use it in if_changed.
.PHONY: ${PHONY}