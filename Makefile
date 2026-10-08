# ====================================================================================
# Setup Project

PROJECT_NAME ?= provider-infisical
PROJECT_REPO ?= github.com/infisical/$(PROJECT_NAME)

export TERRAFORM_VERSION ?= 1.5.7

# Do not allow a version of terraform greater than 1.5.x, due to versions 1.6+ being
# licensed under BSL, which is not permitted.
TERRAFORM_VERSION_VALID := $(shell [ "$(TERRAFORM_VERSION)" = "`printf "$(TERRAFORM_VERSION)\n1.6" | sort -V | head -n1`" ] && echo 1 || echo 0)

export TERRAFORM_PROVIDER_SOURCE ?= Infisical/infisical
export TERRAFORM_PROVIDER_REPO ?= https://github.com/Infisical/terraform-provider-infisical

# DO NOT CHANGE. Version of the Crossplane-specific legacy Terraform build
# ("crossplane-tf-provider/v*" tags). The frozen v1alpha1 API types in
# apis/*/v1alpha1 were generated from its schema. Nothing in the build or the
# provider image uses it any more.
# Keep comments on their own line: make keeps the spaces before an inline
# comment in the value.
export TERRAFORM_CROSSPLANE_SPECIFIC_LEGACY_VERSION ?= 0.0.20

# Version of the normal Terraform provider release (the "v*" tags). The v1alpha2
# API is generated from its schema, and the provider image runs it for all
# resources.
export TERRAFORM_PROVIDER_VERSION ?= 0.20.1
export TERRAFORM_PROVIDER_DOWNLOAD_NAME ?= terraform-provider-infisical
export TERRAFORM_NATIVE_PROVIDER_BINARY ?= terraform-provider-infisical_v$(TERRAFORM_PROVIDER_VERSION)
export TERRAFORM_DOCS_PATH ?= docs/resources
export TERRAFORM_PROVIDER_DOWNLOAD_URL_PREFIX ?= ${TERRAFORM_PROVIDER_REPO}/releases/download/v$(TERRAFORM_PROVIDER_VERSION)

export TERRAFORM_LOCAL_PROVIDER_PATH ?= $(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)/bin
export TERRAFORM_LOCAL_PROVIDER_REPO_PATH ?= $(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)

PLATFORMS ?= linux_amd64 linux_arm64
VERSION ?= v0.1.15

# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Output

-include build/makelib/output.mk

# ====================================================================================
# Setup Go

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_REQUIRED_VERSION ?= 1.26
GOLANGCILINT_VERSION ?= 2.13.0
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/provider $(GO_PROJECT)/cmd/generator
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.Version=$(VERSION)
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.TerraformVersion=$(TERRAFORM_VERSION)
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.TerraformProviderSource=$(TERRAFORM_PROVIDER_SOURCE)
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.TerraformProviderVersion=$(TERRAFORM_PROVIDER_VERSION)
GO_SUBDIRS += cmd internal apis
-include build/makelib/golang.mk

# ====================================================================================
# Setup Kubernetes tools

KIND_VERSION = v0.31.0
UPTEST_VERSION = v2.2.0
CRDDIFF_VERSION = v0.12.1
CROSSPLANE_CLI_VERSION = v2.2.1
-include build/makelib/k8s_tools.mk

# ====================================================================================
# Setup Images

REGISTRY_ORGS ?= xpkg.upbound.io/infisical-inc
IMAGES = $(PROJECT_NAME)
-include build/makelib/imagelight.mk

# ====================================================================================
# Setup XPKG

XPKG_REG_ORGS ?= xpkg.upbound.io/infisical-inc
# NOTE(hasheddan): skip promoting on xpkg.upbound.io as channel tags are
# inferred.
XPKG_REG_ORGS_NO_PROMOTE ?= xpkg.upbound.io/infisical-inc
XPKGS = $(PROJECT_NAME)
-include build/makelib/xpkg.mk

# ====================================================================================
# Fallthrough

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# NOTE(hasheddan): we force image building to happen prior to xpkg build so that
# we ensure image is present in daemon.
xpkg.build.provider-infisical: do.build.images

# NOTE(hasheddan): we ensure up is installed prior to running platform-specific
# build steps in parallel to avoid encountering an installation race condition.
build.init: $(CROSSPLANE_CLI) check-terraform-version

# ====================================================================================
# Setup Terraform for fetching provider schema
TERRAFORM := $(TOOLS_HOST_DIR)/terraform-$(TERRAFORM_VERSION)
TERRAFORM_WORKDIR := $(WORK_DIR)/terraform
TERRAFORM_PROVIDER_SCHEMA := config/schema.json
export TF_CLI_CONFIG_FILE := $(TERRAFORM_WORKDIR)/terraformrc.hcl

check-terraform-version:
ifneq ($(TERRAFORM_VERSION_VALID),1)
	$(error invalid TERRAFORM_VERSION $(TERRAFORM_VERSION), must be less than 1.6.0 since that version introduced a not permitted BSL license))
endif

$(TERRAFORM): check-terraform-version
	@$(INFO) installing terraform $(HOSTOS)-$(HOSTARCH)
	@mkdir -p $(TOOLS_HOST_DIR)/tmp-terraform
	@curl -fsSL https://releases.hashicorp.com/terraform/$(TERRAFORM_VERSION)/terraform_$(TERRAFORM_VERSION)_$(SAFEHOST_PLATFORM).zip -o $(TOOLS_HOST_DIR)/tmp-terraform/terraform.zip
	@unzip $(TOOLS_HOST_DIR)/tmp-terraform/terraform.zip -d $(TOOLS_HOST_DIR)/tmp-terraform
	@mv $(TOOLS_HOST_DIR)/tmp-terraform/terraform $(TERRAFORM)
	@rm -fr $(TOOLS_HOST_DIR)/tmp-terraform
	@$(OK) installing terraform $(HOSTOS)-$(HOSTARCH)

$(TERRAFORM_PROVIDER_SCHEMA): $(TERRAFORM) download-provider-binary
	$(INFO) generating provider schema from GitHub binary
	mkdir -p $(TERRAFORM_WORKDIR)
	cp $(ROOT_DIR)/gen-terraformrc.hcl $(TERRAFORM_WORKDIR)/terraformrc.hcl
	mkdir -p $(TERRAFORM_WORKDIR)/.terraform/plugins/registry.terraform.io/$(TERRAFORM_PROVIDER_SOURCE)/$(TERRAFORM_PROVIDER_VERSION)/$(HOSTOS)_$(SAFEHOSTARCH)
	cp $(WORK_DIR)/$(TERRAFORM_NATIVE_PROVIDER_BINARY) $(TERRAFORM_WORKDIR)/.terraform/plugins/registry.terraform.io/$(TERRAFORM_PROVIDER_SOURCE)/$(TERRAFORM_PROVIDER_VERSION)/$(HOSTOS)_$(SAFEHOSTARCH)/
	echo '{"terraform":[{"required_providers":[{"provider":{"source":"'"$(TERRAFORM_PROVIDER_SOURCE)"'","version":"'"$(TERRAFORM_PROVIDER_VERSION)"'"}}],"required_version":"'"$(TERRAFORM_VERSION)"'"}]}' > $(TERRAFORM_WORKDIR)/main.tf.json
	$(TERRAFORM) -chdir=$(TERRAFORM_WORKDIR) init -upgrade
	$(TERRAFORM) -chdir=$(TERRAFORM_WORKDIR) providers schema -json=true | tee $(TERRAFORM_PROVIDER_SCHEMA)
	$(OK) generating provider schema from GitHub binary

download-provider-binary:
	@$(INFO) downloading provider binary from GitHub releases
	@echo "Downloading from: ${TERRAFORM_PROVIDER_DOWNLOAD_URL_PREFIX}/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_$(TERRAFORM_PROVIDER_VERSION)_$(HOSTOS)_$(SAFEHOSTARCH).zip"
	@mkdir -p $(WORK_DIR)
	@curl -fL -o $(WORK_DIR)/$(TERRAFORM_NATIVE_PROVIDER_BINARY).zip ${TERRAFORM_PROVIDER_DOWNLOAD_URL_PREFIX}/${TERRAFORM_PROVIDER_DOWNLOAD_NAME}_$(TERRAFORM_PROVIDER_VERSION)_$(HOSTOS)_$(SAFEHOSTARCH).zip
	@unzip -o $(WORK_DIR)/$(TERRAFORM_NATIVE_PROVIDER_BINARY).zip -d $(WORK_DIR)
	@chmod +x $(WORK_DIR)/$(TERRAFORM_NATIVE_PROVIDER_BINARY)
	@$(OK) downloaded provider binary from GitHub releases

.PHONY: download-provider-binary

pull-docs:
	@echo "Pulling docs for version v$(TERRAFORM_PROVIDER_VERSION)"
	@# Start from an empty folder, so that docs of other versions do not leak
	@# into config/provider-metadata.yaml.
	@rm -rf "$(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)"
	@mkdir -p "$(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)"
	@git clone -q -c advice.detachedHead=false --depth 1 --filter=blob:none --branch "v$(TERRAFORM_PROVIDER_VERSION)" --sparse "$(TERRAFORM_PROVIDER_REPO)" "$(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)"
	@git -C "$(WORK_DIR)/$(TERRAFORM_PROVIDER_SOURCE)" sparse-checkout set "$(TERRAFORM_DOCS_PATH)"

# The upjet code generator runs goimports on the generated files. Install the
# version pinned by the tool directive in go.mod and put it on the PATH.
GOIMPORTS := $(TOOLS_HOST_DIR)/goimports
export PATH := $(TOOLS_HOST_DIR):$(PATH)

$(GOIMPORTS):
	@$(INFO) installing goimports
	@GOBIN=$(TOOLS_HOST_DIR) go install golang.org/x/tools/cmd/goimports
	@$(OK) installing goimports

generate.init: $(GOIMPORTS) $(TERRAFORM_PROVIDER_SCHEMA) pull-docs

.PHONY: $(TERRAFORM_PROVIDER_SCHEMA) pull-docs check-terraform-version
# ====================================================================================
# Targets

# NOTE: the build submodule currently overrides XDG_CACHE_HOME in order to
# force the Helm 3 to use the .work/helm directory. This causes Go on Linux
# machines to use that directory as the build cache as well. We should adjust
# this behavior in the build submodule because it is also causing Linux users
# to duplicate their build cache, but for now we just make it easier to identify
# its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

# Generate a coverage report for cobertura applying exclusions on
# - generated file
cobertura:
	@cat $(GO_TEST_OUTPUT)/coverage.txt | \
		grep -v zz_ | \
		$(GOCOVER_COBERTURA) > $(GO_TEST_OUTPUT)/cobertura-coverage.xml

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

# This is for running out-of-cluster locally, and is for convenience. Running
# this make target will print out the command which was used. For more control,
# try running the binary directly with different arguments.
run: go.build download-provider-binary
	@$(INFO) Running Crossplane locally out-of-cluster . . .
	mkdir -p $(TERRAFORM_WORKDIR)
	cp $(ROOT_DIR)/local-terraformrc.hcl $(TERRAFORM_WORKDIR)/terraformrc.hcl
	@mkdir -p /tmp/terraform/plugins/registry.terraform.io/$(TERRAFORM_PROVIDER_SOURCE)/$(TERRAFORM_PROVIDER_VERSION)/$(HOSTOS)_$(SAFEHOSTARCH)/
	cp $(WORK_DIR)/$(TERRAFORM_NATIVE_PROVIDER_BINARY) /tmp/terraform/plugins/registry.terraform.io/$(TERRAFORM_PROVIDER_SOURCE)/$(TERRAFORM_PROVIDER_VERSION)/$(HOSTOS)_$(HOSTARCH)/
	@# To see other arguments that can be provided, run the command with --help instead
	UPBOUND_CONTEXT="local" $(GO_OUT_DIR)/provider --debug --poll=60s

# ====================================================================================
# End to End Testing
CROSSPLANE_VERSION ?= 2.2.1
CROSSPLANE_NAMESPACE ?= crossplane-system
-include build/makelib/local.xpkg.mk
-include build/makelib/controlplane.mk

# This target requires the following environment variables to be set:
# - UPTEST_EXAMPLE_LIST, a comma-separated list of examples to test
#   To ensure the proper functioning of the end-to-end test resource pre-deletion hook, it is crucial to arrange your resources appropriately. 
#   You can check the basic implementation here: https://github.com/crossplane/uptest/blob/main/internal/templates/03-delete.yaml.tmpl.
# - UPTEST_CLOUD_CREDENTIALS (optional), multiple sets of AWS IAM User credentials specified as key=value pairs.
#   The support keys are currently `DEFAULT` and `PEER`. So, an example for the value of this env. variable is:
#   DEFAULT='[default]
#   aws_access_key_id = REDACTED
#   aws_secret_access_key = REDACTED'
#   PEER='[default]
#   aws_access_key_id = REDACTED
#   aws_secret_access_key = REDACTED'
#   The associated `ProviderConfig`s will be named as `default` and `peer`.
# - UPTEST_DATASOURCE_PATH (optional), please see https://github.com/crossplane/uptest#injecting-dynamic-values-and-datasource
uptest: $(UPTEST) $(KUBECTL) $(CHAINSAW) $(CROSSPLANE_CLI)
	@$(INFO) running automated tests
	@KUBECTL=$(KUBECTL) CHAINSAW=$(CHAINSAW) CROSSPLANE_CLI=$(CROSSPLANE_CLI) CROSSPLANE_NAMESPACE=$(CROSSPLANE_NAMESPACE) $(UPTEST) e2e "${UPTEST_EXAMPLE_LIST}" --data-source="${UPTEST_DATASOURCE_PATH}" --setup-script=cluster/test/setup.sh --default-conditions="Test" || $(FAIL)
	@$(OK) running automated tests

local-deploy: build controlplane.up local.xpkg.deploy.provider.$(PROJECT_NAME)
	@$(INFO) running locally built provider
	@$(KUBECTL) wait provider.pkg $(PROJECT_NAME) --for condition=Healthy --timeout 5m
	@$(KUBECTL) -n $(CROSSPLANE_NAMESPACE) wait --for=condition=Available deployment --all --timeout=5m
	@$(OK) running locally built provider

e2e: local-deploy uptest

# Crossplane compatibility test. It creates a kind cluster, installs Crossplane
# $(CROSSPLANE_VERSION) and tests the provider that "make build" produced.
#   COMPAT_MODE=fresh    install the local provider package
#   COMPAT_MODE=upgrade  install the released provider, then upgrade it in place
# Set COMPAT_ENV_FILE to a file with INFISICAL_* variables to test against a
# real Infisical instance. See cluster/test/compat.sh for details.
COMPAT_MODE ?= fresh
COMPAT_PROVIDER_IMAGE ?= $(BUILD_REGISTRY)/$(PROJECT_NAME)-$(ARCH)
COMPAT_PROVIDER_XPKG ?= $(XPKG_OUTPUT_DIR)/linux_$(ARCH)/$(PROJECT_NAME)-$(VERSION).xpkg
compat-test: $(KIND) $(HELM) $(KUBECTL) $(CROSSPLANE_CLI)
	@$(INFO) running the Crossplane $(CROSSPLANE_VERSION) compatibility test, mode $(COMPAT_MODE)
	@CROSSPLANE_VERSION=$(CROSSPLANE_VERSION) KIND=$(KIND) HELM=$(HELM) KUBECTL=$(KUBECTL) CROSSPLANE_CLI=$(CROSSPLANE_CLI) \
		KIND_CLUSTER_NAME=infisical-compat-$(subst .,-,$(CROSSPLANE_VERSION))-$(COMPAT_MODE) \
		PROVIDER_IMAGE=$(COMPAT_PROVIDER_IMAGE) PROVIDER_XPKG=$(COMPAT_PROVIDER_XPKG) \
		./cluster/test/compat.sh $(COMPAT_MODE) || $(FAIL)
	@$(OK) running the Crossplane $(CROSSPLANE_VERSION) compatibility test, mode $(COMPAT_MODE)

crddiff: $(UPTEST)
	@$(INFO) Checking breaking CRD schema changes
	@for crd in $${MODIFIED_CRD_LIST}; do \
		if ! git cat-file -e "$${GITHUB_BASE_REF}:$${crd}" 2>/dev/null; then \
			echo "CRD $${crd} does not exist in the $${GITHUB_BASE_REF} branch. Skipping..." ; \
			continue ; \
		fi ; \
		echo "Checking $${crd} for breaking API changes..." ; \
		changes_detected=$$(go run github.com/crossplane/uptest/cmd/crddiff@$(CRDDIFF_VERSION) revision --enable-upjet-extensions <(git cat-file -p "$${GITHUB_BASE_REF}:$${crd}") "$${crd}" 2>&1) ; \
		if [[ $$? != 0 ]] ; then \
			printf "\033[31m"; echo "Breaking change detected!"; printf "\033[0m" ; \
			echo "$${changes_detected}" ; \
			echo ; \
		fi ; \
	done
	@$(OK) Checking breaking CRD schema changes

schema-version-diff:
	@$(INFO) Checking for native state schema version changes
	@export PREV_PROVIDER_VERSION=$$(git cat-file -p "${GITHUB_BASE_REF}:Makefile" | sed -nr 's/^export[[:space:]]*TERRAFORM_PROVIDER_VERSION[[:space:]]*\??=[[:space:]]*(.+)/\1/p'); \
	echo Detected previous Terraform provider version: $${PREV_PROVIDER_VERSION}; \
	echo Current Terraform provider version: $${TERRAFORM_PROVIDER_VERSION}; \
	mkdir -p $(WORK_DIR); \
	git cat-file -p "$${GITHUB_BASE_REF}:config/schema.json" > "$(WORK_DIR)/schema.json.$${PREV_PROVIDER_VERSION}"; \
	./scripts/version_diff.py config/generated.lst "$(WORK_DIR)/schema.json.$${PREV_PROVIDER_VERSION}" config/schema.json
	@$(OK) Checking for native state schema version changes

.PHONY: cobertura submodules fallthrough run crds.clean compat-test

# ====================================================================================
# Special Targets

define CROSSPLANE_MAKE_HELP
Crossplane Targets:
    cobertura             Generate a coverage report for cobertura applying exclusions on generated files.
    submodules            Update the submodules, such as the common build scripts.
    run                   Run crossplane locally, out-of-cluster. Useful for development.

endef
# The reason CROSSPLANE_MAKE_HELP is used instead of CROSSPLANE_HELP is because the crossplane
# binary will try to use CROSSPLANE_HELP if it is set, and this is for something different.
export CROSSPLANE_MAKE_HELP

crossplane.help:
	@echo "$$CROSSPLANE_MAKE_HELP"

help-special: crossplane.help

.PHONY: crossplane.help help-special

# TODO(negz): Update CI to use these targets.
vendor: modules.download
vendor.check: modules.check
