package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_processYAML(t *testing.T) {
	type args struct {
		yamlData []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "simple YAML without annotations",
			args: args{
				yamlData: []byte(`apiVersion: apigateway.aws.upbound.io/v1beta1
kind: BasePathMapping
metadata:
  name: example
spec:
  forProvider:
    region: us-west-1`),
			},
			want: []byte(`apiVersion: apigateway.aws.upbound.io/v1beta1
kind: BasePathMapping
metadata:
  name: example
spec:
  forProvider:
    region: us-west-1
`),
			wantErr: false,
		},
		{
			name: "single YAML with multiple annotations",
			args: args{
				yamlData: []byte(`apiVersion: apigateway.aws.upbound.io/v1beta1
kind: BasePathMapping
metadata:
  annotations:
    upjet.upbound.io/manual-intervention: "The BasePathMapping resource needs a DomainName and DomainName resource needs valid certificates."
    uptest.upbound.io/timeout: "3600" # one hour timeout
    uptest.upbound.io/disable-import: "true"
    uptest.upbound.io/pre-delete-hook: testhooks/delete
  labels:
    testing.upbound.io/example-name: domainname
  name: example-${Rand.RFC1123Subdomain}
spec:
  forProvider:
    region: us-west-1`),
			},
			want: []byte(`apiVersion: apigateway.aws.upbound.io/v1beta1
kind: BasePathMapping
metadata:
  labels:
    testing.upbound.io/example-name: domainname
  name: example-random
spec:
  forProvider:
    region: us-west-1
`),
			wantErr: false,
		},
		{
			name: "multiple YAML with annotations",
			args: args{
				yamlData: []byte(`apiVersion: appconfig.aws.upbound.io/v1beta1
kind: Deployment
metadata:
  annotations:
    upjet.upbound.io/manual-intervention: "Requires configuration version to replaced manually."
    meta.upbound.io/example-id: appconfig/v1beta1/deployment
    crossplane.io/external-name: example
  labels:
    testing.upbound.io/example-name: example
  name: example
spec:
  forProvider:
    region: us-east-1
    tags:
      Type: AppConfig Deployment
---
apiVersion: appconfig.aws.upbound.io/v1beta1
kind: HostedConfigurationVersion
metadata:
  annotations:
    upjet.upbound.io/manual-intervention: "Requires configuration version to replaced manually."
    meta.upbound.io/example-id: appconfig/v1beta1/deployment
  labels:
    testing.upbound.io/example-name: example
  name: example
spec:
  forProvider:
    region: us-east-1
---
apiVersion: appconfig.aws.upbound.io/v1beta1
kind: Application
metadata:
  annotations:
    upjet.upbound.io/manual-intervention: "Requires configuration version to replaced manually."
    uptest.upbound.io/timeout: "5400"
    meta.upbound.io/example-id: appconfig/v1beta1/deployment
  labels:
    testing.upbound.io/example-name: example
  name: example
spec:
  forProvider:
    region: us-east-1`),
			},
			want: []byte(`apiVersion: appconfig.aws.upbound.io/v1beta1
kind: Deployment
metadata:
  annotations:
    crossplane.io/external-name: example
    meta.upbound.io/example-id: appconfig/v1beta1/deployment
  labels:
    testing.upbound.io/example-name: example
  name: example
spec:
  forProvider:
    region: us-east-1
    tags:
      Type: AppConfig Deployment
---
apiVersion: appconfig.aws.upbound.io/v1beta1
kind: HostedConfigurationVersion
metadata:
  annotations:
    meta.upbound.io/example-id: appconfig/v1beta1/deployment
  labels:
    testing.upbound.io/example-name: example
  name: example
spec:
  forProvider:
    region: us-east-1
---
apiVersion: appconfig.aws.upbound.io/v1beta1
kind: Application
metadata:
  annotations:
    meta.upbound.io/example-id: appconfig/v1beta1/deployment
  labels:
    testing.upbound.io/example-name: example
  name: example
spec:
  forProvider:
    region: us-east-1
`),
			wantErr: false,
		},
		{
			name: "single YAML with randomized string without annotation",
			args: args{
				yamlData: []byte(`apiVersion: appconfig.aws.upbound.io/v1beta1
kind: Environment
metadata:
  name: example
spec:
  forProvider:
    name: example-${Rand.RFC1123Subdomain}
    region: us-east-1`),
			},
			want: []byte(`apiVersion: appconfig.aws.upbound.io/v1beta1
kind: Environment
metadata:
  name: example
spec:
  forProvider:
    name: example-random
    region: us-east-1
`),
			wantErr: false,
		},
		{
			name: "invalid YAML",
			args: args{
				yamlData: []byte(`invalid_yaml: [this is not valid`),
			},
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processYAML(tt.args.yamlData)
			if (err != nil) != tt.wantErr {
				t.Errorf("processYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("processYAML() there is a diff: %s", diff)
			}
		})
	}
}
