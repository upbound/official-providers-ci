package templates

import (
	"github.com/crossplane/crossplane-runtime/pkg/test"
	"github.com/google/go-cmp/cmp"
	"github.com/upbound/uptest/internal/config"
	"testing"
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
)

func TestRender(t *testing.T) {
	type args struct {
		tc        *config.TestCase
		resources []config.Resource
	}
	type want struct {
		want map[string]string
		err  error
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
						Name:         "example-bucket",
						KindGroup:    "s3.aws.upbound.io",
						Manifest:     bucketManifest,
						HooksDirPath: "test/bucket-hooks",
						Conditions:   []string{"Test"},
					},
				},
			},
			want: want{
				want: map[string]string{
					"00-apply.yaml": "---\n" + bucketManifest,
					"00-assert.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite
- script: if [ -f test/bucket-hooks/pre.sh ]; then test/bucket-hooks/pre.sh; else echo "No pre hook provided..."; fi
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- script: if [ -f test/bucket-hooks/post.sh ]; then test/bucket-hooks/post.sh; else echo "No post hook provided..."; fi
`,
					"01-delete.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} delete s3.aws.upbound.io/example-bucket --wait=false
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
					Timeout: 10,
				},
				resources: []config.Resource{
					{
						Manifest:     bucketManifest,
						Name:         "example-bucket",
						KindGroup:    "s3.aws.upbound.io",
						HooksDirPath: "test/bucket-hooks",
						Conditions:   []string{"Test"},
					},
					{
						Name:         "test-cluster-claim",
						KindGroup:    "cluster.gcp.platformref.upbound.io",
						Namespace:    "upbound-system",
						Manifest:     claimManifest,
						HooksDirPath: "test/claim-hooks",
						Conditions:   []string{"Ready", "Synced"},
					},
				},
			},
			want: want{
				want: map[string]string{
					"00-apply.yaml": "---\n" + bucketManifest + "---\n" + claimManifest,
					"00-assert.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} annotate managed --all upjet.upbound.io/test=true --overwrite
- script: if [ -f test/bucket-hooks/pre.sh ]; then test/bucket-hooks/pre.sh; else echo "No pre hook provided..."; fi
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=condition=Test --timeout 10s
- script: if [ -f test/bucket-hooks/post.sh ]; then test/bucket-hooks/post.sh; else echo "No post hook provided..."; fi
- script: if [ -f test/claim-hooks/pre.sh ]; then test/claim-hooks/pre.sh; else echo "No pre hook provided..."; fi
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Ready --timeout 10s --namespace upbound-system
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=condition=Synced --timeout 10s --namespace upbound-system
- script: if [ -f test/claim-hooks/post.sh ]; then test/claim-hooks/post.sh; else echo "No post hook provided..."; fi
`,
					"01-delete.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
- command: ${KUBECTL} delete s3.aws.upbound.io/example-bucket --wait=false
- command: ${KUBECTL} delete cluster.gcp.platformref.upbound.io/test-cluster-claim --wait=false --namespace upbound-system
`,
					"01-assert.yaml": `apiVersion: kuttl.dev/v1beta1
kind: TestAssert
timeout: 10
commands:
- command: ${KUBECTL} wait s3.aws.upbound.io/example-bucket --for=delete --timeout 10s
- command: ${KUBECTL} wait cluster.gcp.platformref.upbound.io/test-cluster-claim --for=delete --timeout 10s --namespace upbound-system
- command: ${KUBECTL} wait managed --all --for=delete --timeout 10s
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
			if diff := cmp.Diff(tc.want.want, got); diff != "" {
				t.Errorf("Render(...): -want, +got:\n%s", diff)
			}
		})
	}
}
