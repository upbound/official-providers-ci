# official-providers-ci

> [!IMPORTANT]
> Uptest framework was contributed to CNCF and now located at https://github.com/crossplane/uptest

Repository for the CI tooling of the Upbound official providers repositories:
- `crddiff`: A tool for checking breaking API changes between two CRD OpenAPI v3 schemas. The schemas can come from either two revisions of a CRD, or from the versions declared in a single CRD.
- `buildtagger`: A tool for generating build tags (constraints) for the source modules of the official provider families.
- `lint-provider-family`: A linter for the official provider families. Checks whether all CRDs generated for a provider family are packaged in the corresponding service-scoped provider and checks the provider metadata.
- `perf`: A tool for running performance experiments in the official provider repositories and for collecting & reporting the CPU & Memory utilizations and time to readiness (TTR) for MRs in these experiments.
- `ttr`: A tool that reports the time-to-readiness (TTR) measurements for a subset of the managed resources in a Kubernetes cluster.
- `updoc`: Upbound enhanced document processor.

This repository is also the home of the Upbound official providers reusable workflows:
- `.github/workflows/provider-ci.yml`: A reusable CI workflow for building, linting & validating the official providers.
- `.github/workflows/pr-comment-trigger.yml`: A reusable workflow for triggering `uptest` runs using a specified set of example manifests via pull request comments.
- `.github/workflows/provider-publish-service-artifacts.yml`: A reusable workflow for building the official provider families and pushing their packages to the Upbound registry.
- `.github/workflows/native-provider-bump.yml`: A reusable workflow for bumping the underlying Terraform provider versions of upjet-based official providers.
- `.github/workflows/provider-backport.yml`: A reusable workflow for opening backport PRs in the specified release branches by inspecting the labels on merged PRs.
- `.github/workflows/provider-tag.yml`: A reusable workflow for tagging commits in the release process.
- `.github/workflows/provider-updoc.yml`: A reusable workflow for running `updoc` and publishing the provider documentation to the [Upbound marketplace](https://marketplace.upbound.io/providers).
- `.github/workflows/scan.yml`: A reusable workflow for running [Trivy](https://trivy.dev) scans in the official provider repositories.
- `.github/workflows/provider-commands.yml`: A reusable workflow for opening backport PRs in the specified release branches via PR comments.

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/upbound/uptest/issues).

## Licensing

Uptest is under the Apache 2.0 license.
