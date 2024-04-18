// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package eksdataplane

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type EKSDataplane struct {
	AssumeRole           basetypes.ObjectValue `tfsdk:"assume_role"`
	ClusterConfiguration basetypes.ObjectValue `tfsdk:"cluster_configuration"`
	Status               basetypes.ObjectValue `tfsdk:"status"`
}

type AssumeRole struct {
	RoleArn     basetypes.StringValue `tfsdk:"role_arn"`
	SessionName basetypes.StringValue `tfsdk:"session_name"`
	Region      basetypes.StringValue `tfsdk:"region"`
}

type Status struct {
	ProviderVersion basetypes.StringValue `tfsdk:"provider_version"`
	ProductVersion  basetypes.StringValue `tfsdk:"product_version"`
	UpdatedAt       basetypes.StringValue `tfsdk:"updated_at"`
}

func (m Status) AttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"provider_version": types.StringType,
		"product_version":  types.StringType,
		"updated_at":       types.StringType,
	}
}

type ClusterConfiguration struct {
	Stack       basetypes.StringValue `tfsdk:"stack"`
	DsAccountId basetypes.StringValue `tfsdk:"ds_account_id"`

	AccountId      basetypes.StringValue `tfsdk:"account_id"`
	InfraId        basetypes.StringValue `tfsdk:"infra_id"`
	InfraIndex     basetypes.StringValue `tfsdk:"infra_index"`
	ResourceId     basetypes.StringValue `tfsdk:"resource_id"`
	ProductVersion basetypes.StringValue `tfsdk:"product_version"`

	VpcId             basetypes.StringValue `tfsdk:"vpc_id"`
	VpcCidr           basetypes.StringValue `tfsdk:"vpc_cidr"`
	VpcPrivateSubnets basetypes.ListValue   `tfsdk:"vpc_private_subnets"`

	ClusterIndex        basetypes.Int64Value  `tfsdk:"cluster_index"`
	SubnetIds           basetypes.ListValue   `tfsdk:"subnet_ids"`
	ApiHostname         basetypes.StringValue `tfsdk:"api_hostname"`
	GrafanaHostname     basetypes.StringValue `tfsdk:"grafana_hostname"`
	MetricsPushProxyUrl basetypes.StringValue `tfsdk:"metrics_push_proxy_url"`

	AwsSecretsManagerRoRoleARN  basetypes.StringValue `tfsdk:"aws_secrets_manager_ro_role_arn"`
	InfraManagerRoleArn         basetypes.StringValue `tfsdk:"infra_manager_role_arn"`
	VaultRoleArn                basetypes.StringValue `tfsdk:"vault_role_arn"`
	VaultInitRoleArn            basetypes.StringValue `tfsdk:"vault_init_role_arn"`
	LokiRoleArn                 basetypes.StringValue `tfsdk:"loki_role_arn"`
	TempoRoleArn                basetypes.StringValue `tfsdk:"tempo_role_arn"`
	ThanosStoreGatewayRoleArn   basetypes.StringValue `tfsdk:"thanos_store_gateway_role_arn"`
	ThanosStoreCompactorRoleArn basetypes.StringValue `tfsdk:"thanos_store_compactor_role_arn"`
	ThanosStoreBucketRoleArn    basetypes.StringValue `tfsdk:"thanos_store_bucket_role_arn"`
	ThanosSidecarRoleArn        basetypes.StringValue `tfsdk:"thanos_sidecar_role_arn"`
	DeadmanAlertRoleArn         basetypes.StringValue `tfsdk:"deadman_alert_role_arn"`
	KarpenterRoleArn            basetypes.StringValue `tfsdk:"karpenter_role_arn"`
	KarpenterIrsaRoleArn        basetypes.StringValue `tfsdk:"karpenter_irsa_role_arn"`
	StoreProxyRoleArn           basetypes.StringValue `tfsdk:"store_proxy_role_arn"`
	Cw2LokiRoleArn              basetypes.StringValue `tfsdk:"cw2loki_role_arn"`
	EcrReadonlyRoleArn          basetypes.StringValue `tfsdk:"ecr_readonly_role_arn"`
	DsCrossAccountRoleArn       basetypes.StringValue `tfsdk:"ds_cross_account_role_arn"`
	DpManagerCpRoleArn          basetypes.StringValue `tfsdk:"dp_manager_cp_role_arn"`
	DpManagerRoleArn            basetypes.StringValue `tfsdk:"dp_manager_role_arn"`

	DpOperatorUserAwsSecret basetypes.StringValue `tfsdk:"dp_operator_user_aws_secret"`
	ProductArtifactsBucket  basetypes.StringValue `tfsdk:"product_artifacts_bucket"`

	DefaultInstanceProfile basetypes.StringValue `tfsdk:"default_instance_profile"`
	InterruptionQueueName  basetypes.StringValue `tfsdk:"interruption_queue_name"`
}

func (d *EKSDataplane) AssumeRoleData(ctx context.Context) (AssumeRole, diag.Diagnostics) {
	var ar AssumeRole
	diag := d.AssumeRole.As(ctx, &ar, basetypes.ObjectAsOptions{})
	return ar, diag
}

func (d *EKSDataplane) ClusterConfigurationData(ctx context.Context) (ClusterConfiguration, diag.Diagnostics) {
	var cc ClusterConfiguration
	diag := d.ClusterConfiguration.As(ctx, &cc, basetypes.ObjectAsOptions{})

	if cc.Stack.IsNull() || cc.Stack.IsUnknown() {
		cc.Stack = basetypes.NewStringValue("prod")
	}

	return cc, diag
}
