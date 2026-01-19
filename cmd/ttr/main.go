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

// main package for the ttr tool, which reports the time-to-readiness
// measurements for all managed resources in a cluster.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/crossplane/crossplane-runtime/v2/pkg/fieldpath"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"

	xpv1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
)

// example invocations:
// - ttr -> Report on all managed resources
// - ttr -f cognitoidp.aws.upbound.io/v1beta1/UserPool/example
// - ttr -f //UserPool/ -> Report all UserPool resources
// - ttr -f //UserPool/ -f //VPC/ -> Report all UserPool and VPC resources
// - ttr -f cognitoidp.aws.upbound.io/// -> Report all resources in the group
// - ttr -f ///example-.* -> Report all resources with names prefixed by example-
func main() {
	cf := genericclioptions.NewConfigFlags(true)
	var filters []string
	cmd := &cobra.Command{
		Use:          "ttr",
		Short:        "Reports the time-to-readiness measurements for a subset of the managed resources in a Kubernetes cluster",
		Example:      "ttr --kubeconfig=./kubeconfig",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return report(cf, filters)
		},
	}
	cmd.Flags().StringArrayVarP(&filters, "filters", "f", nil,
		"Zero or more filter expressions each with the following syntax: [group]/[version]/[kind]/[name regex]. Can be repeated. "+
			"Filters managed resources with the specified APIs and names. Missing entries should be specified as empty strings.")
	// add common Kubernetes client configuration flags
	cf.AddFlags(cmd.Flags())
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func report(cf *genericclioptions.ConfigFlags, filters []string) error {
	dc, err := cf.ToDiscoveryClient()
	if err != nil {
		return errors.Wrap(err, "failed to initialize the Kubernetes discovery client")
	}
	c, err := cf.ToRESTConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get REST config for the cluster")
	}
	dyn, err := dynamic.NewForConfig(c)
	if err != nil {
		return errors.Wrap(err, "failed to initialize a dynamic Kubernetes client")
	}
	_, rlList, err := dc.ServerGroupsAndResources()
	if err != nil {
		return errors.Wrap(err, "failed to discover the API resource list")
	}
	f, err := getFilters(filters...)
	if err != nil {
		return errors.Wrap(err, "failed to convert filter expression")
	}
	return errors.Wrap(reportOnAPIs(rlList, f, dyn), "failed to report on the available APIs")
}

func reportOnAPIs(rlList []*metav1.APIResourceList, f filters, dyn dynamic.Interface) error { //nolint:gocyclo // should we break this?
	for _, rl := range rlList {
		for _, r := range rl.APIResources {
			if r.Namespaced {
				continue
			}
			managed := false
			for _, c := range r.Categories {
				if c == "managed" {
					managed = true
					break
				}
			}
			if !managed {
				continue
			}

			gv, err := schema.ParseGroupVersion(rl.GroupVersion)
			if err != nil {
				return errors.Wrapf(err, "failed to parse GroupVersion string: %s", rl.GroupVersion)
			}
			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: r.Name,
			}
			gvk := schema.GroupVersionKind{
				Group:   gv.Group,
				Version: gv.Version,
				Kind:    r.Kind,
			}
			if !f.match(gvk, "") {
				continue
			}

			ri := dyn.Resource(gvr)
			ul, err := ri.List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return errors.Wrapf(err, "failed to list resources with GVR: %s", gvr.String())
			}
			for _, u := range ul.Items {
				if !f.match(gvk, u.GetName()) {
					continue
				}
				reportTTR(gvk, u)
			}
		}
	}
	return nil
}

func reportTTR(gvk schema.GroupVersionKind, u unstructured.Unstructured) {
	rc := getReadyCondition(u)
	// resource not ready yet
	if rc.Status != corev1.ConditionTrue {
		return
	}
	fmt.Printf("%s/%s/%s/%s:%.0f\n", gvk.Group, gvk.Version, gvk.Kind, u.GetName(), rc.LastTransitionTime.Sub(u.GetCreationTimestamp().Time).Seconds())
}

func getReadyCondition(u unstructured.Unstructured) xpv1.Condition {
	conditioned := xpv1.ConditionedStatus{}
	// The path is directly `status` because conditions are inline.
	if err := fieldpath.Pave(u.Object).GetValueInto("status", &conditioned); err != nil {
		return xpv1.Condition{}
	}
	return conditioned.GetCondition(xpv1.TypeReady)
}
