// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/sethvargo/go-retry"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	karpenterv1beta1 "sigs.k8s.io/karpenter/pkg/apis/v1beta1"

	awsconfig "github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/config"
)

var retrylimits = retry.WithMaxRetries(5, retry.NewExponential(time.Second*5))

func getKustomization(ctx context.Context, kubeClient client.Client, name string) (_ *kustomizev1.Kustomization, d diag.Diagnostics) {
	kustomization := &kustomizev1.Kustomization{}
	if err := retry.Do(ctx, retrylimits, func(ctx context.Context) error {
		if err := kubeClient.Get(ctx, client.ObjectKey{Name: name, Namespace: "cluster-config"}, kustomization); err != nil {
			if k8serrors.IsNotFound(err) {
				kustomization = nil
				return nil
			}
			tflog.Debug(ctx, "failed to get "+name+" kustomization "+err.Error())
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		d.AddError("failed to get "+name+" kustomization", err.Error())
		return
	}
	return kustomization, d
}

func deleteKustomization(ctx context.Context, kubeClient client.Client, name string) (d diag.Diagnostics) {
	kustomization, diags := getKustomization(ctx, kubeClient, name)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	if kustomization != nil {
		tflog.Debug(ctx, "Delete "+name+" kustomization")
		if err := retry.Do(ctx, retrylimits, func(ctx context.Context) error {
			if err := kubeClient.Delete(ctx, kustomization, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationForeground)}); err != nil {
				if k8serrors.IsNotFound(err) {
					return nil
				}
				tflog.Debug(ctx, "failed to delete "+name+" kustomization "+err.Error())
				return retry.RetryableError(err)
			}
			return nil
		}); err != nil {
			d.AddError("failed to delete "+name+" kustomization", err.Error())
			return
		}
	}
	return d
}

func suspendKustomization(ctx context.Context, kubeClient client.Client, name string) (d diag.Diagnostics) {
	kustomization, diags := getKustomization(ctx, kubeClient, name)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	if kustomization != nil {
		tflog.Debug(ctx, "Suspend "+name+" kustomization")
		kustomization.Spec.Suspend = true
		if err := retry.Do(ctx, retrylimits, func(ctx context.Context) error {
			err := kubeClient.Update(ctx, kustomization)
			if err != nil {
				tflog.Debug(ctx, "failed to suspend "+name+" kustomization "+err.Error())
				return retry.RetryableError(err)
			}
			return nil
		}); err != nil {
			d.AddError("failed to suspend "+name, err.Error())
			return
		}
	}
	return d
}

func Cleanup(ctx context.Context, cfg aws.Config, dp awsconfig.AWSDataplane, kubeClient client.Client) (d diag.Diagnostics) {
	d.Append(suspendKustomization(ctx, kubeClient, "istio")...)
	if d.HasError() {
		return
	}

	d.Append(suspendKustomization(ctx, kubeClient, "istio-api-ingress")...)
	if d.HasError() {
		return
	}

	d.Append(suspendKustomization(ctx, kubeClient, "istio-grafana-ingress")...)
	if d.HasError() {
		return
	}

	tflog.Debug(ctx, "get list of services in istio namespace")
	svcs := corev1.ServiceList{}
	if err := retry.Do(ctx, retrylimits, func(ctx context.Context) error {
		err := kubeClient.List(ctx, &svcs, client.InNamespace("istio-system"))
		if err != nil {
			tflog.Debug(ctx, "failed to get list of services in istio namespace "+err.Error())
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		d.AddError("failed to list loadbalancer services", err.Error())
		return
	}

	tflog.Debug(ctx, "Delete services in istio namespace")
	for _, svc := range svcs.Items {
		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			continue
		}
		if err := retry.Do(ctx, retrylimits, func(ctx context.Context) error {
			err := kubeClient.Delete(ctx, &svc, &client.DeleteOptions{PropagationPolicy: ptr.To(metav1.DeletePropagationForeground)})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					return nil
				}
				tflog.Debug(ctx, "failed to get list of services in istio namespace "+err.Error())
				return retry.RetryableError(err)
			}
			return nil
		}); err != nil {
			d.AddError("failed to delete loadbalancer "+svc.Name, err.Error())
			return
		}
	}

	d.Append(deleteKustomization(ctx, kubeClient, "data-plane")...)
	if d.HasError() {
		return
	}

	d.Append(suspendKustomization(ctx, kubeClient, "infra")...)
	if d.HasError() {
		return
	}

	kustomizations := kustomizev1.KustomizationList{}
	if err := retry.Do(ctx, retrylimits, func(ctx context.Context) error {
		err := kubeClient.List(ctx, &kustomizations, client.InNamespace("cluster-config"))
		if err != nil {
			tflog.Debug(ctx, "failed to list kustomizations "+err.Error())
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		d.AddError("failed to list kustomizations", err.Error())
		return
	}

	for _, kustomization := range kustomizations.Items {
		if kustomization.Name == "infra" || kustomization.Name == "cilium" || kustomization.Name == "cilium-cluster-policies" || kustomization.Name == "karpenter" || kustomization.Name == "kyverno" || kustomization.Name == "kyverno-policies" {
			continue
		}

		d.Append(deleteKustomization(ctx, kubeClient, kustomization.Name)...)
		if d.HasError() {
			return
		}
	}

	nodeClaims := karpenterv1beta1.NodeClaimList{}
	if err := retry.Do(ctx, retry.WithMaxDuration(time.Minute*20, retry.NewConstant(time.Second*10)), func(ctx context.Context) error {
		err := kubeClient.List(ctx, &nodeClaims)
		if err != nil {
			tflog.Debug(ctx, "failed to list node claims "+err.Error())
			return retry.RetryableError(err)
		}

		for _, nodeClaim := range nodeClaims.Items {
			podList := corev1.PodList{}
			if err := kubeClient.List(ctx, &podList, client.MatchingFields{"spec.nodeName": nodeClaim.Status.NodeName}); err != nil {
				return retry.RetryableError(fmt.Errorf("failed to list pods on node %s: %w", nodeClaim.Status.NodeName, err))
			}

			for _, pod := range podList.Items {
				if err := kubeClient.Delete(ctx, &pod); err != nil {
					return retry.RetryableError(fmt.Errorf("failed to delete pod %s: %w", pod.Name, err))
				}
			}
		}

		tflog.Debug(ctx, "waiting for node claims to be deleted", map[string]any{"count": len(nodeClaims.Items)})
		if len(nodeClaims.Items) > 0 {
			return retry.RetryableError(fmt.Errorf("node claims still exist"))
		}
		return nil
	}); err != nil {
		d.AddError("failed while waiting for node claims to be cleaned up", err.Error())
	}

	// Delete cluster-config secret
	clusterCfg, diags := dp.ClusterConfigurationData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	tflog.Debug(ctx, "Delete cluster settings secret")
	secretsClient := secretsmanager.NewFromConfig(cfg)
	if _, err := secretsClient.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:                   ptr.To(calcDeploymentConfigSecretName(clusterCfg, cfg.Region)),
		ForceDeleteWithoutRecovery: ptr.To(true),
	}); err != nil {
		d.AddError("failed to delete secret", err.Error())
		return
	}

	return
}
