# Makefile for managing the helm charts

chartPath=charts/ccsm-helm

# test: runs the tests without updating the golden files (runs checks against golden files)
.PHONY: test
test:
	go test ./...

# golden: runs the tests with updating the golden files
.PHONY: golden
golden:
	go test ./... -args -update-golden 

# deps: updates and downloads the dependencies for the ccsm helm chart
.PHONY: deps
deps:
	helm dependency update $(chartPath)

# install: install the local ccsm-chart into the current kubernetes cluster/namespace
.PHONY: install
install:
	helm install ccsm-helm-test $(chartPath)

# dry-run: runs an install dry-run with the local ccsm-chart
.PHONY: dry-run
dry-run:
	helm install ccsm-helm-test $(chartPath) --dry-run

# template: show all rendered templates for the local ccsm-chart
.PHONY: template
template:
	helm template ccsm-helm-test $(chartPath)
