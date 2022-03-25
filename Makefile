# Makefile for managing the helm charts

chartPath=charts/ccsm-helm

# test: runs the tests without updating the golden files (runs checks against golden files)
.PHONY: test
test:
	go test ./...

# it: runs the integration tests agains the current kube context
.PHONY: it
it:
	go test -tags integration ./.../integration

# golden: runs the tests with updating the golden files
.PHONY: golden
golden:
	go test ./... -args -update-golden 

# fmt: runs the gofmt in order to format all go files
.PHONY: fmt
fmt:
	go fmt ./... 

# addlicense: add license headers to go files
.PHONY: addlicense
addlicense:
	addlicense -c 'Camunda Services GmbH' -l apache charts/ccsm-helm/test/**/*.go

# checkLicense: checks that the go files contain license header
.PHONY: checkLicense
checkLicense:
	addlicense -check -l apache charts/ccsm-helm/test/**/*.go

# installLicense: installs the addlicense tool
.PHONY: installLicense
installLicense:
	go install github.com/google/addlicense@v1.0.0

#########################################################
######### HELM
#########################################################
# deps: updates and downloads the dependencies for the ccsm helm chart
.PHONY: deps
deps:
	helm dependency update $(chartPath)
	helm dependency update $(chartPath)/charts/identity

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
