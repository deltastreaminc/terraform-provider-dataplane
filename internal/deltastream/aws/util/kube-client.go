// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	helmv2 "github.com/fluxcd/helm-controller/api/v2beta2"
	imageautov1 "github.com/fluxcd/image-automation-controller/api/v1beta1"
	imagereflectv1 "github.com/fluxcd/image-reflector-controller/api/v1beta2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1b3 "github.com/fluxcd/notification-controller/api/v1beta3"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1b2 "github.com/fluxcd/source-controller/api/v1beta2"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/sethvargo/go-retry"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	awsconfig "github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/config"
)

const eksConfigTemplate = `apiVersion: v1
clusters:
- cluster:
    server: {{ .Endpoint }}
    certificate-authority-data: {{ .CAData }}
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: aws
  name: aws
current-context: aws
kind: Config
preferences: {}
users:
- name: aws
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - "eks"
        - "get-token"
        - "--region"
        - "{{ .Region }}"
        - "--cluster-name"
        - "{{ .ClusterName }}"
        - "--output"
        - "json"`

func GetKubeClusterName(ctx context.Context, dp awsconfig.AWSDataplane) (name string, d diag.Diagnostics) {
	clusterConfigurationData, diags := dp.ClusterConfigurationData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	return fmt.Sprintf("dp-%s-%s-%s-%d", clusterConfigurationData.InfraId.ValueString(), clusterConfigurationData.Stack.ValueString(), clusterConfigurationData.EksResourceId.ValueString(), ptr.Deref(clusterConfigurationData.ClusterIndex.ValueInt64Pointer(), 0)), d
}

func DescribeKubeCluster(ctx context.Context, dp awsconfig.AWSDataplane, cfg aws.Config) (cluster *types.Cluster, d diag.Diagnostics) {
	clusterName, diags := GetKubeClusterName(ctx, dp)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	eksClient := eks.NewFromConfig(cfg)
	ekcDescOut, err := eksClient.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(clusterName)})
	if err != nil {
		d.AddError("Failed to describe EKS cluster", err.Error())
		return
	}

	cluster = ekcDescOut.Cluster
	if cluster == nil || cluster.Endpoint == nil || cluster.CertificateAuthority == nil || cluster.CertificateAuthority.Data == nil {
		d.AddError("Failed to get EKS cluster", "Cluster data is nil")
		return
	}
	return cluster, d
}

func GetKubeConfig(ctx context.Context, dp awsconfig.AWSDataplane, cfg aws.Config) (kubeConfig []byte, d diag.Diagnostics) {
	cluster, diags := DescribeKubeCluster(ctx, dp, cfg)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	t, err := template.New("eksConfig").Parse(eksConfigTemplate)
	if err != nil {
		d.AddError("Failed to parse kubeconfig template", err.Error())
		return
	}

	kubeConfigBuf := bytes.NewBuffer(nil)
	err = t.Execute(kubeConfigBuf, map[string]string{
		"Endpoint":    *cluster.Endpoint,
		"CAData":      *cluster.CertificateAuthority.Data,
		"Region":      cfg.Region,
		"ClusterName": *cluster.Name,
	})
	if err != nil {
		d.AddError("Failed to generate kubeconfig", err.Error())
		return
	}
	return kubeConfigBuf.Bytes(), d
}

func GetKubeClient(ctx context.Context, cfg aws.Config, dp awsconfig.AWSDataplane) (kubeClient client.Client, d diag.Diagnostics) {
	kubeconfig, diags := GetKubeConfig(ctx, dp, cfg)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		d.AddError("Failed to connect to kube cluster", err.Error())
		return
	}

	scheme := runtime.NewScheme()
	if err = clientgoscheme.AddToScheme(scheme); err != nil {
		d.AddError("Failed to configure kube client", err.Error())
		return
	}

	apiextensionsv1.AddToScheme(scheme)
	_ = sourcev1b2.AddToScheme(scheme)
	_ = sourcev1.AddToScheme(scheme)
	_ = kustomizev1.AddToScheme(scheme)
	_ = helmv2.AddToScheme(scheme)
	_ = notificationv1.AddToScheme(scheme)
	_ = notificationv1b3.AddToScheme(scheme)
	_ = imagereflectv1.AddToScheme(scheme)
	_ = imageautov1.AddToScheme(scheme)

	kubeClient, err = client.New(restConfig, client.Options{
		Scheme: scheme,
	})
	if err != nil {
		d.AddError("Failed to initialize kube client", err.Error())
		return
	}

	return
}

func ApplyManifests(ctx context.Context, kubeClient client.Client, manifestYamlsCombined string) (d diag.Diagnostics) {
	manifestYamls := strings.Split(manifestYamlsCombined, "\n---\n")
	for _, manifestYaml := range manifestYamls {
		u := &unstructured.Unstructured{}

		if err := yaml.Unmarshal([]byte(manifestYaml), u); err != nil {
			d.AddError("Failed to unmarshal manifest", err.Error())
			return
		}

		tflog.Info(ctx, "Applying object", map[string]any{
			"kind": u.GetKind(),
			"name": u.GetName(),
		})

		if err := retry.Do(ctx, retry.WithMaxRetries(5, retry.NewExponential(time.Second)), func(ctx context.Context) error {
			ug := u.DeepCopy()
			if err := kubeClient.Get(ctx, client.ObjectKeyFromObject(ug), ug); err != nil {
				if k8serrors.IsNotFound(err) {
					if err := kubeClient.Create(ctx, u); err != nil {
						return retry.RetryableError(err)
					}
					return nil
				}
				return retry.RetryableError(err)
			}

			u.SetResourceVersion(ug.GetResourceVersion())
			if err := kubeClient.Update(ctx, u); err != nil {
				return retry.RetryableError(err)
			}
			return nil
		}); err != nil {
			d.AddError("Failed to create manifest", err.Error())
			return
		}
	}
	return
}

func RenderAndApplyTemplate(ctx context.Context, kubeClient client.Client, name string, templateData []byte, data map[string]string) (d diag.Diagnostics) {
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

	return ApplyManifests(ctx, kubeClient, b.String())
}
