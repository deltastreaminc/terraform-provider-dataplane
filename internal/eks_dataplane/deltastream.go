// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package eksdataplane

import (
	"bytes"
	"context"
	_ "embed"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//go:embed assets/flux-system/flux.yaml.tmpl
var fluxManifestTemplate []byte

//go:embed assets/cluster-config/data-plane.yaml.tmpl
var dataPlaneTemplate []byte

//go:embed assets/cluster-config/platform.yaml.tmpl
var platformTemplate []byte

func InstallDeltaStream(ctx context.Context, cfg aws.Config, dp EKSDataplane, kubeClient client.Client) (d diag.Diagnostics) {
	clusterConfig, diags := dp.ClusterConfigurationData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	tflog.Info(ctx, "deploying DeltaStream "+clusterConfig.ProductVersion.ValueString())
	d.Append(renderAndApplyTemplate(ctx, kubeClient, "flux", fluxManifestTemplate, map[string]string{
		"EksReaderRoleArn": clusterConfig.EcrReadonlyRoleArn.ValueString(),
		"Region":           cfg.Region,
		"AccountID":        clusterConfig.AccountId.ValueString(),
	})...)
	if d.HasError() {
		return
	}

	d.Append(renderAndApplyTemplate(ctx, kubeClient, "platform", platformTemplate, map[string]string{
		"Region":         cfg.Region,
		"AccountID":      clusterConfig.AccountId.ValueString(),
		"ProductVersion": clusterConfig.ProductVersion.ValueString(),
	})...)
	if d.HasError() {
		return
	}

	d.Append(renderAndApplyTemplate(ctx, kubeClient, "data plane", dataPlaneTemplate, map[string]string{
		"Region":         cfg.Region,
		"AccountID":      clusterConfig.AccountId.ValueString(),
		"ProductVersion": clusterConfig.ProductVersion.ValueString(),
	})...)
	if d.HasError() {
		return
	}

	deployments := appsv1.DeploymentList{}
	if err := kubeClient.List(ctx, &deployments, client.InNamespace("flux-system")); err != nil {
		d.AddError("error listing flux-system deployments", err.Error())
		return
	}

	for _, deployment := range deployments.Items {
		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = map[string]string{}
		}
		deployment.Spec.Template.Annotations["io.deltastream.tf-deltastream/restartedAt"] = time.Now().Format(time.RFC3339)
		if err := kubeClient.Update(ctx, &deployment); err != nil {
			d.AddError("error updating deployment "+deployment.Name, err.Error())
			return
		}
	}

	return
}

func renderAndApplyTemplate(ctx context.Context, kubeClient client.Client, name string, templateData []byte, data map[string]string) (d diag.Diagnostics) {
	tflog.Debug(ctx, "rendering manifest template "+name)
	t, err := template.New(name).Parse(string(templateData))
	if err != nil {
		d.AddError("error parsing manifest template "+name, err.Error())
		return
	}

	b := bytes.NewBuffer(nil)
	if err := t.Execute(b, data); err != nil {
		d.AddError("error render manifest template "+name, err.Error())
		return
	}

	return applyManifests(ctx, kubeClient, b.String())
}
