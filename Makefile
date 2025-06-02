# Makefile for managing the Helm charts
MAKEFLAGS += --silent
chartPath ?= charts/camunda-platform-*
chartVersion = $(shell grep -Po '(?<=^version: ).+' $(chartPath)/Chart.yaml)
releaseName = camunda-platform-test

#########################################################
######### Go.
#########################################################

#
# Tests.

define go_test_run
	find $(chartPath) -name "go.mod" -exec dirname {} \; | while read chart_dir; do\
		echo "\n[$@] Chart dir: $${chart_dir}";\
		cd $$(git rev-parse --show-toplevel);\
		cd "$${chart_dir}";\
		$(1);\
	done
endef

# go.test: runs the tests without updating the golden files (runs checks against golden files)
.PHONY: go.test
go.test: helm.dependency-update
	@$(call go_test_run, go test ./...)

# go.test-golden-updated: runs the tests with updating the golden files
.PHONY: go.test-golden-updated
go.test-golden-updated: helm.dependency-update
	@$(call go_test_run, go test ./... -args -update-golden)

.PHONY: go.update-golden-only-lite
go.update-golden-only-lite:
	@$(call go_test_run, go test ./...$(APP) -run '^TestGolden.+$$' -args -update-golden)

# go.update-golden-only: update the golden files only without the rest of the tests
.PHONY: go.update-golden-only
go.update-golden-only: helm.dependency-update go.update-golden-only-lite

# go.fmt: runs the gofmt in order to format all go files
.PHONY: go.fmt
go.fmt:
	@$(call go_test_run, go fmt ./...)
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
	addlicense -c 'Camunda Services GmbH' -l apache charts/$(chartPath)/test/**/*.go

# go.addlicense-check: checks that the go files contain license header
.PHONY: go.addlicense-check
go.addlicense-check:
	addlicense -check -l apache charts/$(chartPath)/test/**/*.go

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
	helm repo add elastic https://helm.elastic.co
	helm repo update

# helm.dependency-update: update and downloads the dependencies for the Helm chart
.PHONY: helm.dependency-update
helm.dependency-update:
	find $(chartPath) -name Chart.yaml -exec dirname {} \; | while read chart_dir; do\
		echo "[$@] Chart dir: $${chart_dir}";\
		helm dependency update $${chart_dir};\
	done

# helm.lint: verify that the chart is well-formed.
.PHONY: helm.lint
helm.lint:
	echo "[$@] Chart dir: $(chartPath)"
	helm lint --strict $(chartPath)

# helm.lint: verify that the chart is well-formed.
.PHONY: helm.lint-all
helm.lint-all:
	find $(chartPath) -name Chart.yaml -exec dirname {} \; | while read chart_dir; do\
		$(MAKE) chartPath=$${chart_dir} helm.lint;\
	done

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
	for chart_dir in $(chartPath); do\
		test "camunda-platform-8.2" = "$$(basename $${chart_dir})" && continue;\
		echo "\n[$@] Chart dir: $${chart_dir}";\
		readme-generator \
			--values "$${chart_dir}/values.yaml" \
			--readme "$${chart_dir}/README.md";\
	done

# helm.schema-update: generate schema from values file
.PHONY: helm.schema-update
# TODO: Once 8.7 is released, remove "alpha" name from the excluded versions.
helm.schema-update:
	for chart_dir in $(chartPath); do \
		excluded_versions="camunda-platform-(8\.(2|3|4|5|6|7)|alpha)$$"; \
		if echo "$${chart_dir}" | grep -qE "$${excluded_versions}"; then \
			echo "\n[$@] Chart dir: $${chart_dir}";\
			echo "[$@] This chart version doesn't have schema";\
			continue; \
		fi; \
		echo "\n[$@] Chart dir: $${chart_dir}"; \
		readme-generator \
			--values "$${chart_dir}/values.yaml" \
			--schema "$${chart_dir}/values.schema.json";\
	done

# helm.get-images: list all images in the chart.
.PHONY: helm.get-images
helm.get-images:
	export CHART_SOURCE="$(chartSource)"; \
	export CHART_VERSION="$(chartVersion)"; \
	export CHART_VALUES_DIR="$(chartPath)/"; \
	bash -x scripts/list-chart-images.sh;

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

.PHONY: release.generate-notes
release.generate-notes:
	for chart_dir in $(chartPath); do\
		echo "\n[$@] Chart dir: $${chart_dir}";\
		bash scripts/generate-release-notes.sh --main "$${chart_dir}";\
	done

.PHONY: release.generate-notes-footer
release.generate-notes-footer:
	for chart_dir in $(chartPath); do\
		echo "\n[$@] Chart dir: $${chart_dir}";\
		bash scripts/generate-release-notes.sh --footer "$${chart_dir}";\
	done

.PHONY: release.generate-and-commit
release.generate-and-commit: release.generate-notes
	git add $(chartPath);\
	git commit -m "chore: add generated files for camunda-platform $(chartVersion)"

.PHONY: release.verify-components-version
release.verify-components-version:
	@bash scripts/verify-components-version.sh

.PHONY: release.generate-version-matrix-index
release.generate-version-matrix-index:
	@bash scripts/generate-version-matrix.sh --init
	@bash scripts/generate-version-matrix.sh --index

.PHONY: release.generate-version-matrix-released
release.generate-version-matrix-released:
	@bash scripts/generate-version-matrix.sh --init
	@bash scripts/generate-version-matrix.sh --released

.PHONY: release.generate-version-matrix-unreleased
release.generate-version-matrix-unreleased:
	@bash scripts/generate-version-matrix.sh --init
	@bash scripts/generate-version-matrix.sh --unreleased

.PHONY: release.set-prs-version-label
release.set-prs-version-label:
	@bash scripts/set-prs-version-label.sh

#########################################################
######### Precommit
#########################################################

.PHONY: precommit.chores
precommit.chores: helm.lint helm.readme-update go.update-golden-only-lite
