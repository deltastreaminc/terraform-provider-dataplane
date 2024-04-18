// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package eksdataplane

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func GetAwsConfig(ctx context.Context, dp EKSDataplane) (cfg aws.Config, d diag.Diagnostics) {
	assumeRoleData, diags := dp.AssumeRoleData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	cfgOpts := config.WithClientLogMode(aws.LogDeprecatedUsage)
	cfg, err := config.LoadDefaultConfig(ctx, cfgOpts)
	if err != nil {
		d.AddError("Failed to load AWS SDK config", err.Error())
		return
	}
	if !assumeRoleData.Region.IsUnknown() && !assumeRoleData.Region.IsNull() {
		cfg.Region = assumeRoleData.Region.ValueString()
	}

	stsClient := sts.NewFromConfig(cfg)
	creds := stscreds.NewAssumeRoleProvider(stsClient, assumeRoleData.RoleArn.ValueString(), func(o *stscreds.AssumeRoleOptions) {
		if !assumeRoleData.SessionName.IsUnknown() && !assumeRoleData.SessionName.IsNull() {
			o.RoleSessionName = assumeRoleData.SessionName.ValueString()
		}
	})
	cfg.Credentials = creds
	return cfg, d
}
