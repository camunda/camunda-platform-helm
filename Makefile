# Makefile for managing the helm charts

chartPath=charts/ccsm-helm
releaseName=ccsm-helm-test

# test: runs the tests without updating the golden files (runs checks against golden files)
.PHONY: test
test:	deps
	go test ./...

# it: runs the integration tests agains the current kube context
.PHONY: it
it:	deps
	go test -timeout 1h -tags integration ./.../integration

# golden: runs the tests with updating the golden files
.PHONY: golden
golden:	deps
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
install:	deps
	helm install $(releaseName) $(chartPath)

# uninstall: uninstalls the ccsm-chart and removes all related pvc's
.PHONY: uninstall
uninstall:
	-helm uninstall $(releaseName)
	-kubectl delete pvc -l app.kubernetes.io/instance=$(releaseName)
	-kubectl delete pvc -l app=elasticsearch-master

# dry-run: runs an install dry-run with the local ccsm-chart
.PHONY: dry-run
dry-run:	deps
	helm install $(releaseName) $(chartPath) --dry-run

# template: show all rendered templates for the local ccsm-chart
.PHONY: template
template:	deps
	helm template $(releaseName) $(chartPath)

#########################################################
######### Testing
#########################################################

.PHONY: topology
topology:
	kubectl exec svc/$(releaseName)-zeebe-gateway -- zbctl --insecure status
