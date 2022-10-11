# UPTEST

The end to end integration testing tool for Crossplane providers and configurations.

Uptest comes as a binary which can be installed from the releases section. It runs end-to-end tests
by applying the provided examples and waiting for the expected conditions. Other than that, it enables templating to
insert dynamic values into the examples and supports running scripts as hooks just before and right after applying
the examples.

## Usage

```shell
uptest --help
usage: uptest [<flags>]

Automated Test Tool for Upbound Official Providers

Flags:
  --help                       Show context-sensitive help (also try --help-long and --help-man).
  --example-list=EXAMPLE-LIST  List of example manifests. Value of this option will be used to trigger/configure the tests.The
                               possible usage: 'examples/s3/bucket.yaml,examples/iam/user.yaml': The
                               comma separated resources are used as test inputs. If this option is not set, 'EXAMPLE_LIST' env var
                               is used as default.
  --data-source=""             File path of data source that will be used for injection some values.
  --hooks-directory="test/hooks"
                               Path to hooks directory where `pre.sh` and/or `post.sh` may exist.
  --default-timeout=1200       Default timeout in seconds for the test.
  --claim-or-composite         Resource to test is either claim or composite instead of a managed resource
```

Examples can be provided as a comma separated list via `--example-list` flag or `EXAMPLE_LIST` env var.
Hooks can be provided as a directory via `--hooks-directory` flag or `HOOKS_DIRECTORY` env var. It defaults to
`test/hooks` directory and it executes `pre.sh` and/or `post.sh` scripts in that directory if they exist.

By default, *uptest* assumes the example is a managed resource generated with [upjet](https://github.com/upbound/upjet).
If the example is a claim or a composite resource `--claim-or-composite` flag should be set.

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/upbound/uptest/issues).

## Licensing

uptest is under the Apache 2.0 license.
