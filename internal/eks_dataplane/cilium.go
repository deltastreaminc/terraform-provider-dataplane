// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package eksdataplane

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/deltastreaminc/terraform-provider-deltastream-dataplane/internal/eks_dataplane/helm"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/sethvargo/go-retry"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed assets/cilium-1.15.1.tgz
var ciliumChart []byte

//go:embed assets/cilium-values.yaml.tmpl
var ciliumValuesTemplate string

func InstallCilium(ctx context.Context, cfg aws.Config, dp EKSDataplane, kubeClient client.Client) (d diag.Diagnostics) {
	kubeConfig, diags := GetKubeConfig(ctx, dp, cfg)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	config, diags := dp.ClusterConfigurationData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	clusterName, diags := GetKubeClusterName(ctx, dp)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	b := bytes.NewBuffer(nil)
	t, err := template.New("cilium-values").Parse(ciliumValuesTemplate)
	if err != nil {
		d.AddError("error parsing cilium values template", err.Error())
		return
	}
	if err = t.Execute(b, map[string]any{
		"ClusterName":     clusterName,
		"EcrAwsAccountId": config.AccountId.ValueString(),
		"Region":          cfg.Region,
	}); err != nil {
		d.AddError("error executing cilium values template", err.Error())
		return
	}

	if err = helm.InstallRelease(ctx, kubeConfig, "kube-system", "cilium", bytes.NewBuffer(ciliumChart), b.Bytes(), true); err != nil {
		d.AddError("error installing cilium release", err.Error())
		return
	}

	tflog.Debug(ctx, "cilium installed, wait for nodes to be ready")
	err = retry.Do(ctx, retry.WithMaxDuration(time.Minute*5, retry.NewConstant(time.Second*5)), func(ctx context.Context) error {
		nodes := corev1.NodeList{}
		if err = kubeClient.List(ctx, &nodes); err != nil {
			return retry.RetryableError(err)
		}

		ready := true
		for _, node := range nodes.Items {
			for _, c := range node.Status.Conditions {
				if c.Type == corev1.NodeReady && c.Status != corev1.ConditionTrue {
					ready = false
					break
				}
			}
		}

		if !ready {
			return retry.RetryableError(fmt.Errorf("nodes not ready"))
		}
		return nil
	})
	if err != nil {
		d.AddError("timeout waiting for nodes to be ready", err.Error())
		return
	}
	tflog.Debug(ctx, "nodes are ready")

	return
}

func DeleteAwsNode(ctx context.Context, dp EKSDataplane, kubeClient client.Client) (d diag.Diagnostics) {
	nodeRequiresRestart := false
	awsNodeDS := &appsv1.DaemonSet{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: "kube-system", Name: "aws-node"}, awsNodeDS); err != nil {
		if !k8serrors.IsNotFound(err) {
			d.AddError("error getting aws-node DaemonSet", err.Error())
			return
		}
		tflog.Debug(ctx, "aws-node daemonset not found")
		awsNodeDS = nil
	}
	if awsNodeDS != nil {
		nodeRequiresRestart = true
		tflog.Debug(ctx, "deleting aws-node daemonset")
		if err := kubeClient.Delete(ctx, awsNodeDS); err != nil {
			d.AddError("error deleting aws-node DaemonSet", err.Error())
			return
		}
	}
	awsNodeSA := &corev1.ServiceAccount{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Namespace: "kube-system", Name: "aws-node"}, awsNodeSA); err != nil {
		if !k8serrors.IsNotFound(err) {
			d.AddError("error getting aws-node DaemonSet", err.Error())
			return
		}
		tflog.Debug(ctx, "aws-node service account not found")
		awsNodeSA = nil
	}
	if awsNodeSA != nil {
		nodeRequiresRestart = true
		tflog.Debug(ctx, "deleting aws-node service account")
		if err := kubeClient.Delete(ctx, awsNodeSA); err != nil {
			d.AddError("error deleting aws-node DaemonSet", err.Error())
			return
		}
	}

	if nodeRequiresRestart {
		if diags := restartNodes(ctx, dp, kubeClient); diags.HasError() {
			d.Append(diags...)
			return
		}
	}

	return
}
