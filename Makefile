# Makefile for managing the Helm charts

chartPath=charts/camunda-platform
chartVersion=$(shell grep -Po '(?<=^version: ).+' $(chartPath)/Chart.yaml)
releaseName=camunda-platform-test

#########################################################
######### Go.
#########################################################

#
# Tests.

# go.test: runs the tests without updating the golden files (runs checks against golden files)
.PHONY: go.test
go.test: helm.dependency-update
	go test ./...

# go.test-golden-updated: runs the tests with updating the golden files
.PHONY: go.test-golden-updated
go.test-golden-updated: helm.dependency-update
	go test ./... -args -update-golden

# go.update-golden-only: update the golden files only without the rest of the tests
.PHONY: go.update-golden-only
go.update-golden-only: helm.dependency-update
	go test ./...$(APP) -run '^TestGolden.+$$' -args -update-golden

# go.fmt: runs the gofmt in order to format all go files
.PHONY: go.fmt
go.fmt:
	go fmt ./...
	@diff=$$(git status --porcelain | grep -F ".go" || true)
	@if [ -n "$${diff}" ]; then\
		echo "Some files are not following the go format ($${diff}), run gofmt and fix your files.";\
		exit 1;\
	fi

#
# Helpers.

# go.addlicense-install: installs the addlicense tool
.PHONY: go.addlicense-install
go.addlicense-install:
	go install github.com/google/addlicense@v1.0.0

# go.addlicense-run: adds license headers to go files
.PHONY: go.addlicense-run
go.addlicense-run:
	addlicense -c 'Camunda Services GmbH' -l apache charts/camunda-platform/test/**/*.go

# go.addlicense-check: checks that the go files contain license header
.PHONY: go.addlicense-check
go.addlicense-check:
	addlicense -check -l apache charts/camunda-platform/test/**/*.go

#########################################################
######### Tools
#########################################################

# Add asdf plugins.
.tools.asdf-plugins-add:
	@# Add plugins from .tool-versions file within the repo.
	@# If the plugin is already installed asdf exits with 2, so grep is used to handle that.
	@for plugin in $$(awk '{print $$1}' .tool-versions); do \
		echo "$${plugin}"; \
		asdf plugin add $${plugin} 2>&1 | (grep "already added" && exit 0); \
	done

# Install tools via asdf.
tools.asdf-install: .tools.asdf-plugins-add
	asdf install

# This target will be mainly used in the CI to update the images tag from camunda-platform repo
.PHONY: tools.update-values-file-image-tag
tools.update-values-file-image-tag:
	@bash scripts/update-values-file-image-tag.sh

.PHONY: tools.zbctl-topology
tools.zbctl-topology:
	kubectl exec svc/$(releaseName)-zeebe-gateway -- zbctl --insecure status

#########################################################
######### HELM
#########################################################

# helm.repos-add: add Helm repos needed by the charts
.PHONY: helm.repos-add
helm.repos-add:
	helm repo add camunda https://helm.camunda.io
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm repo update

# helm.dependency-update: update and downloads the dependencies for the Helm chart
.PHONY: helm.dependency-update
helm.dependency-update:
	helm dependency update $(chartPath)

# helm.install: install the local chart into the current kubernetes cluster/namespace
.PHONY: helm.install
helm.install: helm.dependency-update
	helm install $(releaseName) $(chartPath)

# helm.uninstall: uninstall the chart and removes all related pvc's
.PHONY: helm.uninstall
helm.uninstall:
	-helm uninstall $(releaseName)
	-kubectl delete pvc -l app.kubernetes.io/instance=$(releaseName)
	-kubectl delete pvc -l release=$(releaseName)

# helm.dry-run: run an install dry-run with the local chart
.PHONY: helm.dry-run
helm.dry-run: helm.dependency-update
	helm install $(releaseName) $(chartPath) --dry-run

# helm.template: show all rendered templates for the local chart
.PHONY: helm.template
helm.template: helm.dependency-update
	helm template $(releaseName) $(chartPath)

# helm.readme-update: generate readme from values file
.PHONY: helm.readme-update
helm.readme-update:
	readme-generator \
		--values "$(chartPath)/values.yaml" \
		--readme "$(chartPath)/README.md"

#########################################################
######### Release
#########################################################

.PHONY: .release.bump-chart-version
.release.bump-chart-version:
	@bash scripts/bump-chart-version.sh

.PHONY: release.bump-chart-version-and-commit
release.bump-chart-version-and-commit: .release.bump-chart-version
	git add $(chartPath);\
	git commit -m "chore: bump camunda-platform chart version to $(chartVersion)"

.PHONY: .release.generate-notes
.release.generate-notes:
	@bash scripts/generate-release-notes.sh

.PHONY: release.generate-and-commit
release.generate-and-commit: .release.generate-notes
	git add $(chartPath);\
	git commit -m "chore: add generated files for camunda-platform $(chartVersion)"

.PHONY: release.generate-pr-url
release.generate-pr-url:
	@echo "\n\n###################################\n"
	@echo "Open the release PR using this URL:"
	@echo "https://github.com/camunda/camunda-platform-helm/compare/main...release?expand=1&template=release_template.md&title=Release%20Camunda%20Platform%20Helm%20Chart%20v$(chartVersion)&labels=camunda-platform,chore"
	@echo "\n###################################\n\n"

.PHONY: release.chores
release.chores:
	git checkout main
	git pull --tags
	git switch -C release
	@$(MAKE) release.bump-chart-version-and-commit
	@$(MAKE) release.generate-and-commit
	git push -fu origin release
	@$(MAKE) release.generate-pr-url
	git checkout main

.PHONY: release.verify-components-version
release.verify-components-version:
	@bash scripts/verify-components-version.sh

.PHONY: release.generate-version-matrix-single
release.generate-version-matrix-single:
	@bash scripts/generate-version-matrix.sh --init
	@bash scripts/generate-version-matrix.sh --single

.PHONY: release.generate-version-matrix-all
release.generate-version-matrix-all:
	@bash scripts/generate-version-matrix.sh --init
	@bash scripts/generate-version-matrix.sh --all
