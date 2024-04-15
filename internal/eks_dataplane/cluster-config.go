// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package eksdataplane

import (
	"context"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func UpdateClusterConfig(ctx context.Context, cfg aws.Config, dp EKSDataplane, kubeClient client.Client, infraVersion string) (d diag.Diagnostics) {
	ns := &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: "cluster-config"}}
	controllerutil.CreateOrUpdate(ctx, kubeClient, ns, func() error {
		return nil
	})

	config, diags := dp.ClusterConfigurationData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	cluster, diags := DescribeKubeCluster(ctx, dp, cfg)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	promPushProxyUri, err := url.Parse(config.MetricsPushProxyUrl.ValueString())
	if err != nil {
		d.AddError("error parsing cpPrometheusPushProxyUrl", err.Error())
		return
	}

	vpcPrivateSubnets := []string{}
	d.Append(config.VpcPrivateSubnets.ElementsAs(ctx, &vpcPrivateSubnets, false)...)
	if d.HasError() {
		return
	}

	clusterSubnetIds := []string{}
	d.Append(config.SubnetIds.ElementsAs(ctx, &clusterSubnetIds, false)...)
	if d.HasError() {
		return
	}

	clusterConfig := corev1.Secret{ObjectMeta: v1.ObjectMeta{Name: "cluster-settings", Namespace: "cluster-config"}}
	controllerutil.CreateOrUpdate(ctx, kubeClient, &clusterConfig, func() error {
		clusterConfig.Data = map[string][]byte{
			"meshID":                         []byte("deltastream"),
			"stack":                          []byte(config.Stack.ValueString()),
			"cloud":                          []byte("aws"),
			"region":                         []byte(cfg.Region),
			"topology":                       []byte("dp"),
			"dsEcrAccountID":                 []byte(config.AccountId.ValueString()),
			"awsAccountID":                   []byte(config.AccountId.ValueString()),
			"infraID":                        []byte(config.InfraId.ValueString()),
			"infraName":                      []byte("dp-" + config.InfraId.ValueString()),
			"infraIndex":                     []byte(config.InfraIndex.ValueString()),
			"infraRegion":                    []byte(cfg.Region),
			"infraDrRegion":                  []byte(cfg.Region),
			"infraEnvironment":               []byte(config.Stack.ValueString()),
			"infraCloud":                     []byte("aws"),
			"infraVersion":                   []byte(infraVersion),
			"resourceID":                     []byte(config.ResourceId.ValueString()),
			"clusterName":                    []byte(*cluster.Name),
			"vpcId":                          []byte(config.VpcId.ValueString()),
			"vpcCidr":                        []byte(config.VpcCidr.ValueString()),
			"vpcPrivateSubnetIDs":            []byte(strings.Join(vpcPrivateSubnets, ",")),
			"clusterPrivateSubnetIDs":        []byte(strings.Join(clusterSubnetIds, ",")),
			"discoveryRegion":                []byte(cfg.Region),
			"apiServerURI":                   []byte(*cluster.Endpoint),
			"apiServerTokenIssuer":           []byte(*cluster.Identity.Oidc.Issuer),
			"loadbalancerClass":              []byte("service.k8s.aws/nlb"),
			"autoscaleMin":                   []byte("3"),
			"autoscaleMax":                   []byte("5"),
			"externalSecretsRoleARN":         []byte(config.AwsSecretsManagerRoRoleARN.ValueString()),
			"infraOperatorRoleARN":           []byte(config.InfraManagerRoleArn.ValueString()),
			"vaultRoleARN":                   []byte(config.VaultRoleArn.ValueString()),
			"vaultInitRoleARN":               []byte(config.VaultInitRoleArn.ValueString()),
			"lokiRoleARN":                    []byte(config.LokiRoleArn.ValueString()),
			"tempoRoleARN":                   []byte(config.TempoRoleArn.ValueString()),
			"thanosStoreGatewayRoleARN":      []byte(config.ThanosStoreGatewayRoleArn.ValueString()),
			"thanosStoreCompactorRoleARN":    []byte(config.ThanosStoreCompactorRoleArn.ValueString()),
			"thanosStoreBucketWebRoleARN":    []byte(config.ThanosStoreBucketRoleArn.ValueString()),
			"thanosSideCarRoleARN":           []byte(config.ThanosSidecarRoleArn.ValueString()),
			"deadmanAlertRoleARN":            []byte(config.DeadmanAlertRoleArn.ValueString()),
			"karpenterRoleARN":               []byte(config.KarpenterRoleArn.ValueString()),
			"karpenterIrsaARN":               []byte(config.KarpenterIrsaRoleArn.ValueString()),
			"storeProxyRoleARN":              []byte(config.StoreProxyRoleArn.ValueString()),
			"datagenRoleARN":                 []byte(""),
			"defaultInstanceProfile":         []byte(config.DefaultInstanceProfile.ValueString()),
			"interruptionQueueName":          []byte(config.InterruptionQueueName.ValueString()),
			"cw2lokiRoleARN":                 []byte(config.Cw2LokiRoleArn.ValueString()),
			"nthRoleARN":                     []byte(""),
			"nthCordonOnly":                  []byte("true"),
			"dpManagerCPAssumeRoleARN":       []byte(config.DpManagerCpRoleArn.ValueString()),
			"dpManagerRoleARN":               []byte(config.DpManagerRoleArn.ValueString()),
			"dpOperatorUserAwsSecret":        []byte(config.DpOperatorUserAwsSecret.ValueString()),
			"deltastreamCrossAccountRoleARN": []byte(config.DsCrossAccountRoleArn.ValueString()),
			"apiHostname":                    []byte(config.ApiHostname.ValueString()),
			"grafanaHostname":                []byte(config.GrafanaHostname.ValueString()),
			"cpPrometheusPushProxyUrl":       []byte(config.MetricsPushProxyUrl.ValueString()),
			"cpPrometheusPushProxyHost":      []byte(promPushProxyUri.Hostname()),
			"cpPrometheusPushProxyPort":      []byte(`"443"`),
			"grafanaPromPushProxVpcHostname": []byte(""),
			"grafanaVpcHostname":             []byte(config.GrafanaHostname.ValueString()),
		}
		return nil
	})

	return
}
