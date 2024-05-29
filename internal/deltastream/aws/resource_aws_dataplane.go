// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/deltastreaminc/terraform-provider-dataplane/internal/config"
	awsconfig "github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/config"
	"github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/util"
)

var _ resource.Resource = &AWSDataplaneResource{}
var _ resource.ResourceWithConfigure = &AWSDataplaneResource{}

func NewAWSDataplaneResource() resource.Resource {
	return &AWSDataplaneResource{}
}

type AWSDataplaneResource struct {
	infraVersion string
}

// Schema implements resource.Resource.
func (d *AWSDataplaneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = awsconfig.Schema
}

func (d *AWSDataplaneResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	cfg, ok := req.ProviderData.(*config.DataplaneResourceData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *DeltaStreamProviderCfg, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.infraVersion = cfg.Version
}

func (d *AWSDataplaneResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws"
}

// Create implements resource.Resource.
func (d *AWSDataplaneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var dp awsconfig.AWSDataplane

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &dp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := util.GetAwsConfig(ctx, dp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	kubeClient, diags := util.GetKubeClient(ctx, cfg, dp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// copy images
	resp.Diagnostics.Append(CopyImages(ctx, cfg, dp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// remove aws-node
	resp.Diagnostics.Append(DeleteAwsNode(ctx, dp, kubeClient)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// install cilium
	resp.Diagnostics.Append(InstallCilium(ctx, cfg, dp, kubeClient)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// update cluster-config
	resp.Diagnostics.Append(UpdateClusterConfig(ctx, cfg, dp, kubeClient, d.infraVersion)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// start microservices
	resp.Diagnostics.Append(InstallDeltaStream(ctx, cfg, dp, kubeClient)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterConfig, diags := dp.ClusterConfigurationData(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	status := &awsconfig.Status{
		ProviderVersion: basetypes.NewStringValue(d.infraVersion),
		ProductVersion:  clusterConfig.ProductVersion,
		LastModified:    basetypes.NewStringValue(time.Now().Format(time.RFC3339)),
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

func (d *AWSDataplaneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var dp awsconfig.AWSDataplane

	resp.Diagnostics.Append(req.State.Get(ctx, &dp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := util.GetAwsConfig(ctx, dp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	kubeClient, diags := util.GetKubeClient(ctx, cfg, dp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(Cleanup(ctx, cfg, dp, kubeClient)...)
}

func (d *AWSDataplaneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var newDp awsconfig.AWSDataplane

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &newDp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg, diags := util.GetAwsConfig(ctx, newDp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	kubeClient, diags := util.GetKubeClient(ctx, cfg, newDp)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// // update cluster-config
	resp.Diagnostics.Append(UpdateClusterConfig(ctx, cfg, newDp, kubeClient, d.infraVersion)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// copy images
	resp.Diagnostics.Append(CopyImages(ctx, cfg, newDp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// start microservices
	resp.Diagnostics.Append(InstallDeltaStream(ctx, cfg, newDp, kubeClient)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterConfig, diags := newDp.ClusterConfigurationData(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	status := &awsconfig.Status{
		ProviderVersion: basetypes.NewStringValue(d.infraVersion),
		ProductVersion:  clusterConfig.ProductVersion,
		LastModified:    basetypes.NewStringValue(time.Now().Format(time.RFC3339)),
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

func (d *AWSDataplaneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var dp awsconfig.AWSDataplane

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &dp)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, dp)...)
}
