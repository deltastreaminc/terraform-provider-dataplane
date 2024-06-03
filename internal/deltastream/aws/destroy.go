// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	awsconfig "github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/config"
)

func Cleanup(ctx context.Context, cfg aws.Config, dp awsconfig.AWSDataplane, kubeClient client.Client) (d diag.Diagnostics) {
	clusterCfg, diags := dp.ClusterConfigurationData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	// Delete vault secret
	secretsClient := secretsmanager.NewFromConfig(cfg)
	if _, err := secretsClient.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(fmt.Sprintf(
			"deltastream/%s/dp/%s/aws/%s/vault",
			clusterCfg.Stack.ValueString(),
			clusterCfg.InfraId.ValueString(),
			cfg.Region,
		)),
	}); err != nil {
		d.AddError("failed to delete secret", err.Error())
		return
	}

	istioKustomization := &kustomizev1.Kustomization{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: "istio", Namespace: "cluster-config"}, istioKustomization); err != nil {
		if k8serrors.IsNotFound(err) {
			return
		}
		d.AddError("failed to get lookup service", err.Error())
		return
	}

	istioKustomization.Spec.Suspend = true
	if err := kubeClient.Update(ctx, istioKustomization); err != nil {
		d.AddError("failed to suspend service", err.Error())
		return
	}

	svcs := corev1.ServiceList{}
	if err := kubeClient.List(ctx, &svcs, client.InNamespace("istio-system")); err != nil {
		d.AddError("failed to list services", err.Error())
		return
	}
	for _, svc := range svcs.Items {
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}
		if err := kubeClient.Delete(ctx, &svc); err != nil {
			d.AddError("failed to delete loadbalancer service "+svc.Name, err.Error())
			return
		}
	}

	dataplaneKustomization := &kustomizev1.Kustomization{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: "data-plane", Namespace: "cluster-config"}, dataplaneKustomization); err != nil {
		if !k8serrors.IsNotFound(err) {
			d.AddError("failed to get lookup service", err.Error())
			return
		}
	} else {
		if err := kubeClient.Delete(ctx, dataplaneKustomization); err != nil {
			d.AddError("failed to delete data-plane services", err.Error())
			return
		}
	}

	infraKustomization := &kustomizev1.Kustomization{}
	if err := kubeClient.Get(ctx, client.ObjectKey{Name: "infra", Namespace: "cluster-config"}, infraKustomization); err != nil {
		if !k8serrors.IsNotFound(err) {
			d.AddError("failed to get lookup service", err.Error())
			return
		}
	} else {
		if err := kubeClient.Delete(ctx, infraKustomization); err != nil {
			d.AddError("failed to delete infra services", err.Error())
			return
		}
	}

	// Delete cluster-config secret
	if _, err := secretsClient.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId: ptr.To(calcDeploymentConfigSecretName(clusterCfg, cfg.Region)),
	}); err != nil {
		d.AddError("failed to delete secret", err.Error())
		return
	}

	return
}
