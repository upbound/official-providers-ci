# Project Setup
PROJECT_NAME := uptest
PROJECT_REPO := github.com/upbound/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64
include build/makelib/common.mk

# Setup Go
GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd
GO_LDFLAGS += -X $(GO_PROJECT)/pkg/version.Version=$(VERSION)
include build/makelib/golang.mk
