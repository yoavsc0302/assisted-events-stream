BUILD_IMAGE ?= assisted-events-streams:latest
NAMESPACE ?= assisted-events-streams
IMAGE_NAME ?= quay.io/edge-infrastructure/assisted-events-stream
IMAGE_TAG ?= latest
KIND_CLUSTER_NAME ?= assisted-events-streams
REDIS_IMAGE_NAME ?= "quay.io/edge-infrastructure/redis"
REDIS_IMAGE_TAG ?= 6.2.7-debian-10-r23
REDIS_EXPORTER_IMAGE_NAME ?= "quay.io/edge-infrastructure/redis-exporter"
REDIS_EXPORTER_IMAGE_TAG ?= 1.37.0-debian-10-r63

.PHONY: all
all: validate

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: docker-build
docker-build: ## Build docker image
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) -f Dockerfile .

.PHONY: redis-docker-build
redis-docker-build: ## Build docker image
	docker build -t $(REDIS_IMAGE_NAME):$(REDIS_IMAGE_TAG) -f Dockerfile.redis .
	docker build -t $(REDIS_EXPORTER_IMAGE_NAME):$(REDIS_EXPORTER_IMAGE_TAG) -f Dockerfile.redis-exporter .

.PHONY: kind-create-cluster
kind-create-cluster: ## Create kind cluster
	kind create cluster --name $(KIND_CLUSTER_NAME) 2>/dev/null ||:

.PHONY: kind-load-docker
kind-load-docker: docker-build ## Load docker image into the kind cluster
	kind load docker-image --name $(KIND_CLUSTER_NAME) $(IMAGE_NAME):$(IMAGE_TAG)

.PHONY: deploy-all
deploy-all: kind-create-cluster kind-load-docker ## Deploy locally all necessary components
	oc create --save-config ns ${NAMESPACE} ||:
	oc kustomize manifest/overlays/dev/ | oc process -p IMAGE_NAME=$(IMAGE_NAME) -p IMAGE_TAG=$(IMAGE_TAG) --local -f - | oc apply -n $(NAMESPACE) -f -

.PHONY: generate-template
generate-template: ## Generate template in openshift/template.yaml
	oc kustomize manifest/overlays/production/ > openshift/template.yaml

.PHONY: generate-mocks
generate-mocks: ## Generate mocks
	find -name 'mock_*' -delete
	go generate ./...

.PHONY: unit-test
unit-test: generate-mocks ## Run unit tests
	ginkgo -r

.PHONY: lint
lint:	
	golangci-lint run -v

.PHONY: format
format:
	golangci-lint run --fix -v

.PHONY: validate
validate: lint unit-test
