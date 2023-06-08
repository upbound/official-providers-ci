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

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/upbound/upjet/pkg/migration"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	unstructuredAwsVpc = map[string]interface{}{
		"apiVersion": "ec2.aws.upbound.io/v1beta1",
		"kind":       "VPC",
		"metadata": map[string]interface{}{
			"name": "sample-vpc",
		},
	}
	unstructuredAwsProviderConfig = map[string]interface{}{
		"apiVersion": "aws.upbound.io/v1beta1",
		"kind":       "ProviderConfig",
		"metadata": map[string]interface{}{
			"name": "sample-pc",
		},
	}
	unstructuredAzureZone = map[string]interface{}{
		"apiVersion": "network.azure.upbound.io/v1beta1",
		"kind":       "Zone",
		"metadata": map[string]interface{}{
			"name": "sample-zone",
		},
	}
	unstructuredAzureResourceGroup = map[string]interface{}{
		"apiVersion": "azure.upbound.io/v1beta1",
		"kind":       "ResourceGroup",
		"metadata": map[string]interface{}{
			"name": "example-resources",
		},
	}
	unstructuredGcpZone = map[string]interface{}{
		"apiVersion": "network.gcp.upbound.io/v1beta1",
		"kind":       "Zone",
		"metadata": map[string]interface{}{
			"name": "sample-zone",
		},
	}
	unstructuredGcpProviderConfig = map[string]interface{}{
		"apiVersion": "gcp.upbound.io/v1beta1",
		"kind":       "ProviderConfig",
		"metadata": map[string]interface{}{
			"name": "sample-pc",
		},
	}
	unstructuredInvalidProviderConfig = map[string]interface{}{
		"apiVersion": "xyz.invalid.upbound.io/v1beta1",
		"kind":       "Kind",
		"metadata": map[string]interface{}{
			"name": "sample-pc",
		},
	}
)

func TestGetSSOPNameFromManagedResource(t *testing.T) {
	type args struct {
		u migration.UnstructuredWithMetadata
	}
	type want struct {
		ssopMap map[string]struct{}
		err     error
	}

	cases := map[string]struct {
		args
		want
	}{
		"Aws": {
			args: args{
				u: migration.UnstructuredWithMetadata{
					Object: unstructured.Unstructured{
						Object: unstructuredAwsVpc,
					},
				},
			},
			want: want{
				ssopMap: map[string]struct{}{
					"provider-aws-ec2": {},
				},
				err: nil,
			},
		},
		"Family-Aws": {
			args: args{
				u: migration.UnstructuredWithMetadata{
					Object: unstructured.Unstructured{
						Object: unstructuredAwsProviderConfig,
					},
				},
			},
			want: want{
				ssopMap: map[string]struct{}{},
				err:     nil,
			},
		},
		"Azure": {
			args: args{
				u: migration.UnstructuredWithMetadata{
					Object: unstructured.Unstructured{
						Object: unstructuredAzureZone,
					},
				},
			},
			want: want{
				ssopMap: map[string]struct{}{
					"provider-azure-network": {},
				},
				err: nil,
			},
		},
		"Family-Azure": {
			args: args{
				u: migration.UnstructuredWithMetadata{
					Object: unstructured.Unstructured{
						Object: unstructuredAzureResourceGroup,
					},
				},
			},
			want: want{
				ssopMap: map[string]struct{}{},
				err:     nil,
			},
		},
		"Gcp": {
			args: args{
				u: migration.UnstructuredWithMetadata{
					Object: unstructured.Unstructured{
						Object: unstructuredGcpZone,
					},
				},
			},
			want: want{
				ssopMap: map[string]struct{}{
					"provider-gcp-network": {},
				},
				err: nil,
			},
		},
		"Family-Gcp": {
			args: args{
				u: migration.UnstructuredWithMetadata{
					Object: unstructured.Unstructured{
						Object: unstructuredGcpProviderConfig,
					},
				},
			},
			want: want{
				ssopMap: map[string]struct{}{},
				err:     nil,
			},
		},
		"InvalidProvider": {
			args: args{
				u: migration.UnstructuredWithMetadata{
					Object: unstructured.Unstructured{
						Object: unstructuredInvalidProviderConfig,
					},
				},
			},
			want: want{
				ssopMap: map[string]struct{}{},
				err:     nil,
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			SSOPNames = map[string]struct{}{}
			err := GetSSOPNameFromManagedResource(tc.u)
			if diff := cmp.Diff(tc.want.err, err); diff != "" {
				t.Errorf("\nNext(...): -want, +got:\n%s", diff)
			}
			if diff := cmp.Diff(tc.want.ssopMap, SSOPNames); diff != "" {
				t.Errorf("\nNext(...): -want, +got:\n%s", diff)
			}
		})
	}
}
