// Copyright 2023 Upbound Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package templates

import (
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"

	"github.com/upbound/uptest/internal/config"
)

const (
	bucketManifest = `apiVersion: s3.aws.crossplane.io/v1beta1
kind: Bucket
metadata:
  name: test-bucket
spec:
  deletionPolicy: Delete
`

	claimManifest = `apiVersion: gcp.platformref.upbound.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster-claim
  namespace: upbound-system
spec:
  parameters:
    nodes:
      count: 1
      size: small
`

	secretManifest = `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: upbound-system
type: Opaque
data:
  key: dmFsdWU=
`
)

func TestRender(t *testing.T) {
	type args struct {
		tc        *config.TestCase
		resources []config.Resource
	}
	type want struct {
		out map[string]string
		err error
	}
	tests := map[string]struct {
		args args
		want want
	}{
		"SuccessSingleResource": {
			args: args{
				tc: &config.TestCase{
					Timeout: 10,
				},
				resources: []config.Resource{
					{
						Name:       "example-bucket",
						KindGroup:  "s3.aws.upbound.io",
						YAML:       bucketManifest,
						Conditions: []string{"Test"},
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": "---\n" + bucketManifest,
					"00-assert.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
`,
					"01-delete.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} delete s3.aws.upbound.io/example-bucket --wait=false --ignore-not-found
`,
					"01-assert.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=delete --timeout 10s
- command: ${KUBECTL} wait managed --all --for=delete --timeout 10s
`,
				},
			},
		},
		"SuccessMultipleResource": {
			args: args{
				tc: &config.TestCase{
					Timeout:            10,
					SetupScriptPath:    "/tmp/setup.sh",
					TeardownScriptPath: "/tmp/teardown.sh",
				},
				resources: []config.Resource{
					{
						YAML:                 bucketManifest,
						Name:                 "example-bucket",
						KindGroup:            "s3.aws.upbound.io",
						PreAssertScriptPath:  "/tmp/bucket/pre-assert.sh",
						PostDeleteScriptPath: "/tmp/bucket/post-delete.sh",
						Conditions:           []string{"Test"},
					},
					{
						YAML:                 claimManifest,
						Name:                 "test-cluster-claim",
						KindGroup:            "cluster.gcp.platformref.upbound.io",
						Namespace:            "upbound-system",
						PostAssertScriptPath: "/tmp/claim/post-assert.sh",
						PreDeleteScriptPath:  "/tmp/claim/pre-delete.sh",
						Conditions:           []string{"Ready", "Synced"},
					},
					{
						YAML:      secretManifest,
						Name:      "test-secret",
						KindGroup: "secret.",
						Namespace: "upbound-system",
					},
				},
			},
			want: want{
				out: map[string]string{
					"00-apply.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: /tmp/setup.sh
` + "---\n" + bucketManifest + "---\n" + claimManifest + "---\n" + secretManifest,
					"00-assert.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite
- command: /tmp/bucket/pre-assert.sh
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Ready --timeout 10s --namespace upbound-system
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Synced --timeout 10s --namespace upbound-system
- command: /tmp/claim/post-assert.sh
`,
					"01-delete.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} delete s3.aws.upbound.io/example-bucket --wait=false --ignore-not-found
- command: /tmp/bucket/post-delete.sh
- command: /tmp/claim/pre-delete.sh
- command: ${KUBECTL} delete cluster.gcp.platformref.upbound.io/test-cluster-claim --wait=false --namespace upbound-system --ignore-not-found
`,
					"01-assert.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=delete --timeout 10s
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=delete --timeout 10s --namespace upbound-system
- command: ${KUBECTL} wait managed --all --for=delete --timeout 10s
- command: /tmp/teardown.sh
`,
				},
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := Render(tc.args.tc, tc.args.resources)
			if diff := cmp.Diff(tc.want.err, err, test.EquateErrors()); diff != "" {
				t.Errorf("Render(...): -want error, +got error:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.out, got); diff != "" {
				t.Errorf("Render(...): -want, +got:\n%s", diff)
			}
		})
	}
}
