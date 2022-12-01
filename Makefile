# Project Setup
PROJECT_NAME := uptest
PROJECT_REPO := github.com/upbound/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64 darwin_amd64 darwin_arm64
-include build/makelib/common.mk

-include build/makelib/output.mk

# Setup Go
GO_REQUIRED_VERSION = 1.19
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd
GO_SUBDIRS += cmd
GO111MODULE = on
-include build/makelib/golang.mk

fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

.PHONY: submodules fallthrough

-include build/makelib/k8s_tools.mk
-include build/makelib/controlplane.mk

uptest:
	@echo "Running uptest"
	@printenv