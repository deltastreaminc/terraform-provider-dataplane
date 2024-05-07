// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"time"

	eksdataplane "github.com/deltastreaminc/terraform-provider-deltastream-dataplane/internal/eks_dataplane"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = &EKSDataplaneResource{}
var _ resource.ResourceWithConfigure = &EKSDataplaneResource{}

func NewEKSDataplaneResource() resource.Resource {
	return &EKSDataplaneResource{}
}

type EKSDataplaneResource struct {
	infraVersion string
}

func (d *EKSDataplaneResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "EKS Dataplane resource",

		Attributes: map[string]schema.Attribute{
			"assume_role": schema.SingleNestedAttribute{
				Description: "Assume role configuration",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"role_arn": schema.StringAttribute{
						Description: "Amazon Resource Name (ARN) of an IAM Role to assume prior to making API calls.",
						Optional:    true,
					},
					"session_name": schema.StringAttribute{
						Description: "An identifier for the assumed role session.",
						Optional:    true,
					},
					"region": schema.StringAttribute{
						Description: "The AWS region to use for the assume role.",
						Optional:    true,
					},
				},
			},
			"cluster_configuration": schema.SingleNestedAttribute{
				Description: "Cluster configuration",
				Required:    true,
				Attributes: map[string]schema.Attribute{
					"stack": schema.StringAttribute{
						Description: "The type of DeltaStream dataplane (default: prod).",
						Optional:    true,
					},
					"ds_account_id": schema.StringAttribute{
						Description: "The account ID provided by DeltaStream.",
						Required:    true,
					},

					"account_id": schema.StringAttribute{
						Description: "The account ID hosting the DeltaStream dataplane.",
						Required:    true,
					},
					"product_version": schema.StringAttribute{
						Description: "The version of the DeltaStream product. (provided by DeltaStream)",
						Required:    true,
					},

					"infra_id": schema.StringAttribute{
						Description: "The infra ID of the DeltaStream dataplane (provided by DeltaStream).",
						Required:    true,
					},
					"infra_index": schema.StringAttribute{
						Description: "The infra index of the DeltaStream dataplane (provided by DeltaStream).",
						Required:    true,
					},
					"resource_id": schema.StringAttribute{
						Description: "The resource ID of the DeltaStream dataplane (provided by DeltaStream).",
						Required:    true,
					},
					"cluster_index": schema.Int64Attribute{
						Description: "The index of the cluster (provided by DeltaStream).",
						Optional:    true,
					},
					"subnet_ids": schema.ListAttribute{
						Description: "The private subnet IDs hosting nodes for this cluster.",
						ElementType: basetypes.StringType{},
						Required:    true,
					},

					"aws_secrets_manager_ro_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for reading secrets from AWS secrets manager.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"infra_manager_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing infra resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"vault_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for credential vault resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"vault_init_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for configuring credential vault.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"loki_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing Loki resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"tempo_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing Tempo resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"thanos_store_gateway_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing Thanos storage gateway resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"thanos_store_compactor_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing Thanos storage compactor resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"thanos_store_bucket_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing Thanos store bucket resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"thanos_sidecar_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing Thanos sidecar resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"deadman_alert_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing deadman alert resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"karpenter_role_name": schema.StringAttribute{
						Description: "The name of the role to assume for managing Karpenter resources.",
						Required:    true,
					},
					"karpenter_irsa_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing Karpenter IRSA resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"store_proxy_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume to facilitate connection to customer stores.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"cw2loki_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing CloudWatch-Loki resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"ds_cross_account_role_arn": schema.StringAttribute{
						Description: "The ARN of the role for provising trust when accessing customer provided resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"ecr_readonly_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for read-only access to ECR.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"dp_manager_cp_role_arn": schema.StringAttribute{
						Description: "The ARN of the control plane role to assume for data plane to control plane communication (provided by DeltaStream)",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"dp_manager_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing dataplane resources.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},
					"interruption_queue_name": schema.StringAttribute{
						Description: "The name of the SQS queue for handling interruption events.",
						Required:    true,
					},
					"metrics_push_proxy_url": schema.StringAttribute{
						Description: "The URL of the metrics push proxy.",
						Required:    true,
					},
					"vpc_id": schema.StringAttribute{
						Description: "The VPC ID of the cluster.",
						Required:    true,
					},
					"vpc_dns_ip": schema.StringAttribute{
						Description: "The VPC DNS server IP address.",
						Required:    true,
						Validators:  []validator.String{},
					},
					"vpc_cidr": schema.StringAttribute{
						Description: "The CIDR of the VPC.",
						Required:    true,
					},
					"vpc_private_subnets": schema.ListAttribute{
						Description: "The private subnet IDs of the private links from dataplane VPC.",
						ElementType: basetypes.StringType{},
						Required:    true,
					},
					"product_artifacts_bucket": schema.StringAttribute{
						Description: "The S3 bucket for storing DeltaStream product artifacts.",
						Required:    true,
					},
					"workload_credentials_mode": schema.StringAttribute{
						Description: "The mode for managing workload credentials.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.OneOf("secret", "role")},
					},
					"workload_credentials_secret": schema.StringAttribute{
						Description: "The name of the secret containing workload credentials if running in secret mode.",
						Optional:    true,
					},
					"workload_credentials_role_arn": schema.StringAttribute{
						Description: "The ARN of the role to assume for managing workload credentials if running in role iammode.",
						Optional:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:role/.+$`), "Invalid Role ARN")},
					},

					"o11y_hostname": schema.StringAttribute{
						Description: "The hostname of the observability endpoint.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-zA-Z0-9-\.]+\.[a-zA-Z]{2,}$`), "Invalid hostname")},
					},
					"o11y_subnet_mode": schema.StringAttribute{
						Description: "The subnet mode for observability endpoint.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.OneOf("public", "private")},
					},
					"o11y_tls_mode": schema.StringAttribute{
						Description: "The TLS/HTTPS mode for observability endpoint.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.OneOf("awscert", "acme", "disabled")},
					},
					"o11y_tls_certificate_arn": schema.StringAttribute{
						Description: "The ARN of the TLS certificate for the observability endpoint.",
						Optional:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:certificate/.+$`), "Invalid Certificate ARN")},
					},

					"api_hostname": schema.StringAttribute{
						Description: "The hostname of the dataplane API endpoint.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[a-zA-Z0-9-\.]+\.[a-zA-Z]{2,}$`), "Invalid hostname")},
					},
					"api_subnet_mode": schema.StringAttribute{
						Description: "The subnet mode for dataplane API endpoint.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.OneOf("public", "private")},
					},
					"api_tls_mode": schema.StringAttribute{
						Description: "The TLS/HTTPS mode for dataplane API endpoint.",
						Required:    true,
						Validators:  []validator.String{stringvalidator.OneOf("awscert", "acme", "disabled")},
					},
					"api_tls_certificate_arn": schema.StringAttribute{
						Description: "The ARN of the TLS certificate for the dataplane API endpoint.",
						Optional:    true,
						Validators:  []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^arn:aws:iam::[0-9]{12}:certificate/.+$`), "Invalid Certificate ARN")},
					},
				},
			},
			"status": schema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]schema.Attribute{
					"provider_version": schema.StringAttribute{
						Description: "The version of the DeltaStream provider used to install the dataplane.",
						Computed:    true,
					},
					"product_version": schema.StringAttribute{
						Description: "The version of the DeltaStream product installed on the dataplane.",
						Computed:    true,
					},
					"updated_at": schema.StringAttribute{
						Description: "The time the dataplane was last updated.",
						Computed:    true,
					},
				},
			},
		},
	}
}

func (d *EKSDataplaneResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	cfg, ok := req.ProviderData.(*DeltaStreamDataplaneResourceData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *DeltaStreamProviderCfg, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.infraVersion = cfg.Version
}

func (d *EKSDataplaneResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_eks"
}

// Create implements resource.Resource.
func (d *EKSDataplaneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var dp eksdataplane.EKSDataplane

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &dp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := eksdataplane.GetAwsConfig(ctx, dp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	kubeClient, diags := eksdataplane.GetKubeClient(ctx, cfg, dp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// copy images
	resp.Diagnostics.Append(eksdataplane.CopyImages(ctx, cfg, dp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// remove aws-node
	resp.Diagnostics.Append(eksdataplane.DeleteAwsNode(ctx, dp, kubeClient)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// install cilium
	resp.Diagnostics.Append(eksdataplane.InstallCilium(ctx, cfg, dp, kubeClient)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// update cluster-config
	resp.Diagnostics.Append(eksdataplane.UpdateClusterConfig(ctx, cfg, dp, kubeClient, d.infraVersion)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// start microservices
	resp.Diagnostics.Append(eksdataplane.InstallDeltaStream(ctx, cfg, dp, kubeClient)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterConfig, diags := dp.ClusterConfigurationData(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	status := &eksdataplane.Status{
		ProviderVersion: basetypes.NewStringValue(d.infraVersion),
		ProductVersion:  clusterConfig.ProductVersion,
		UpdatedAt:       basetypes.NewStringValue(time.Now().Format(time.RFC3339)),
	}
	dp.Status, diags = basetypes.NewObjectValueFrom(ctx, status.AttributeTypes(), status)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &dp)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *EKSDataplaneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var dp eksdataplane.EKSDataplane

	resp.Diagnostics.Append(req.State.Get(ctx, &dp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := eksdataplane.GetAwsConfig(ctx, dp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	kubeClient, diags := eksdataplane.GetKubeClient(ctx, cfg, dp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(eksdataplane.Cleanup(ctx, cfg, dp, kubeClient)...)
}

func (d *EKSDataplaneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var newDp eksdataplane.EKSDataplane

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &newDp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := eksdataplane.GetAwsConfig(ctx, newDp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// copy images
	resp.Diagnostics.Append(eksdataplane.CopyImages(ctx, cfg, newDp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	kubeClient, diags := eksdataplane.GetKubeClient(ctx, cfg, newDp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// // update cluster-config
	resp.Diagnostics.Append(eksdataplane.UpdateClusterConfig(ctx, cfg, newDp, kubeClient, d.infraVersion)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// start microservices
	resp.Diagnostics.Append(eksdataplane.InstallDeltaStream(ctx, cfg, newDp, kubeClient)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterConfig, diags := newDp.ClusterConfigurationData(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	status := &eksdataplane.Status{
		ProviderVersion: basetypes.NewStringValue(d.infraVersion),
		ProductVersion:  clusterConfig.ProductVersion,
		UpdatedAt:       basetypes.NewStringValue(time.Now().Format(time.RFC3339)),
	}
	newDp.Status, diags = basetypes.NewObjectValueFrom(ctx, status.AttributeTypes(), status)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newDp)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (d *EKSDataplaneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var dp eksdataplane.EKSDataplane

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &dp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, dp)...)
}
