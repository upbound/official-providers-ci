# Provider Tools

## Uptest

The end to end integration testing tool for Crossplane providers and configurations.

Uptest comes as a binary which can be installed from the releases section. It runs end-to-end tests
by applying the provided examples and waiting for the expected conditions. Other than that, it enables templating to
insert dynamic values into the examples and supports running scripts as hooks just before and right after applying
the examples.

### Usage

```shell
$ uptest e2e --help
usage: uptest e2e [<flags>] [<manifest-list>]

Run e2e tests for manifests by applying them to a control plane and waiting until a given condition is met.

Flags:
  --help                        Show context-sensitive help (also try --help-long and --help-man).
  --data-source=""              File path of data source that will be used for injection some values.
  --setup-script=""             Script that will be executed before running tests.
  --teardown-script=""          Script that will be executed after running tests.
  --default-timeout=1200        Default timeout in seconds for the test. Timeout could be overridden per resource using
                                "uptest.upbound.io/timeout" annotation.
  --default-conditions="Ready"  Comma seperated list of default conditions to wait for a successful test. Conditions could be
                                overridden per resource using "uptest.upbound.io/conditions" annotation.

Args:
  [<manifest-list>]  List of manifests. Value of this option will be used to trigger/configure the tests.The possible usage:
                     'provider-aws/examples/s3/bucket.yaml,provider-gcp/examples/storage/bucket.yaml': The comma separated resources
                     are used as test inputs. If this option is not set, 'MANIFEST_LIST' env var is used as default.
```

Uptest expects a running control-plane (a.k.a. k8s + crossplane) where required providers are running and/or required
configuration were applied.

#### Example: 

```shell
uptest e2e examples/user.yaml,examples/bucket.yaml --setup-script="test/hooks/setup.sh"
```

#### Hooks

There are 4 types of hooks that can be used to customize the test flow:

1. `setup-script`: This hook will be executed before running the tests case. It is useful to set up the control plane
   before running the tests. For example, you can use it to create a provider config and your cloud credentials. This
   can be configured via `--setup-script` flag as a relative path to where uptest is executed.
2. `teardown-script`: This hook will be executed after running the tests case. This can be configured via
   `--teardown-script` flag as a relative path to where uptest is executed.
3. `pre-assert-hook`: This hook will be executed before running the assertions and after applying a specific manifest.
    This can be configured via `uptest.upbound.io/pre-assert-hook` annotation on the manifest as a relative path to the
    manifest file.
4. `post-assert-hook`: This hook will be executed after running the assertions. This can be configured via 
    `uptest.upbound.io/post-assert-hook` annotation on the manifest as a relative path to the manifest file.

> All hooks need to be executables, please make sure to set the executable bit on your scripts, e.g. with `chmod +x`.

#### Troubleshooting

Uptest uses [kuttl](https://kuttl.dev/) under the hood and generates a `kuttl` test case based on the provided input.
You can inspect the generated kuttl test case by checking the temporary test directory which is printed in the beginning
of uptest e2e output. For example:

```shell
Running kuttl tests at /var/folders/_5/jc7399qx6cn6t5535npv9z4c0000gn/T/uptest-e2e
```

# Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/upbound/uptest/issues).

# Licensing

All tools in this repository are under the Apache 2.0 license.
