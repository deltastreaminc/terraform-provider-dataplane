// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	_ "embed"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/sethvargo/go-retry"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	awsconfig "github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/config"
	"github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/util"
)

//go:embed assets/custom-credentials.yaml
var customCredentialKustomization []byte

func deployCustomCredentialsContiner(ctx context.Context, cfg aws.Config, dp awsconfig.AWSDataplane) (d diag.Diagnostics) {
	clusterConfig, diags := dp.ClusterConfigurationData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	kubeClient, err := util.GetKubeClient(ctx, cfg, dp)
	if err != nil {
		d.AddError("error getting kube client", err.Error())
		return
	}

	if clusterConfig.CustomCredentialsImage.IsNull() || clusterConfig.CustomCredentialsImage.IsUnknown() {
		return
	}

	imgSpl := strings.Split(clusterConfig.CustomCredentialsImage.ValueString(), ":")
	if len(imgSpl) != 2 {
		d.AddError("invalid custom credentials image", clusterConfig.CustomCredentialsImage.ValueString())
		return
	}

	d.Append(util.RenderAndApplyTemplate(ctx, kubeClient, "custom credentials", customCredentialKustomization, map[string]string{
		"Region":          cfg.Region,
		"AccountID":       clusterConfig.AccountId.ValueString(),
		"ImageRepository": imgSpl[0],
		"ImageTag":        imgSpl[1],
	})...)
	if d.HasError() {
		return
	}

	if err = retry.Do(ctx, retry.WithMaxDuration(time.Minute*5, retry.NewConstant(time.Second*5)), func(ctx context.Context) error {
		kubeClient, err := util.GetKubeClient(ctx, cfg, dp)
		if err != nil {
			return retry.RetryableError(err)
		}

		dpmanagerDeployment := &appsv1.Deployment{ObjectMeta: v1.ObjectMeta{Name: "dp-manager", Namespace: "deltastream"}}
		if err = kubeClient.Get(ctx, client.ObjectKeyFromObject(dpmanagerDeployment), dpmanagerDeployment); err != nil {
			return retry.RetryableError(err)
		}

		if dpmanagerDeployment.Spec.Template.Annotations == nil {
			dpmanagerDeployment.Spec.Template.Annotations = map[string]string{}
		}
		dpmanagerDeployment.Spec.Template.Annotations["dataplane.deltastream.io/rollout"] = time.Now().String()

		if err = kubeClient.Update(ctx, dpmanagerDeployment); err != nil {
			return retry.RetryableError(err)
		}
		return nil
	}); err != nil {
		d.AddError("error updating dp-manager deployment", err.Error())
		return
	}

	return
}
