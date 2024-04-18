// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package eksdataplane

import (
	"context"
	"net/url"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func restartNodes(ctx context.Context, dp EKSDataplane, kubeClient client.Client) (d diag.Diagnostics) {
	cfg, diags := GetAwsConfig(ctx, dp)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	clusterName, diags := GetKubeClusterName(ctx, dp)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	eksClient := eks.NewFromConfig(cfg)
	ec2Client := ec2.NewFromConfig(cfg)

	tflog.Debug(ctx, "listing node groups")
	nodegroupsOutput, err := eksClient.ListNodegroups(ctx, &eks.ListNodegroupsInput{
		ClusterName: &clusterName,
	})
	if err != nil {
		d.AddError("error listing nodegroups", err.Error())
		return
	}
	tflog.Debug(ctx, "found node groups", map[string]any{"nodegroups": nodegroupsOutput.Nodegroups})

	for _, nodegroupName := range nodegroupsOutput.Nodegroups {
		nodes := corev1.NodeList{}
		if err = kubeClient.List(ctx, &nodes, client.MatchingLabels{"eks.amazonaws.com/nodegroup": nodegroupName}); err != nil {
			d.AddError("error listing nodes in nodegroup", err.Error())
			return
		}

		instanceIDs := []string{}
		for _, node := range nodes.Items {
			u, err := url.Parse(node.Spec.ProviderID)
			if err != nil {
				d.AddError("error parsing node provider ID: "+node.Spec.ProviderID, err.Error())
				return
			}
			instanceIDs = append(instanceIDs, filepath.Base(u.Path))
		}
		tflog.Debug(ctx, "found instances in node group", map[string]any{"nodegroup": nodegroupName, "instances": instanceIDs})

		_, err := ec2Client.RebootInstances(ctx, &ec2.RebootInstancesInput{
			InstanceIds: instanceIDs,
		})
		if err != nil {
			d.AddError("error rebooting instances", err.Error())
			return
		}
		tflog.Debug(ctx, "rebooted instances", map[string]any{"nodegroup": nodegroupName, "instances": instanceIDs})
	}
	return
}
