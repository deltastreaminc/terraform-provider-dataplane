package aws

import (
	"bytes"
	"context"
	"html/template"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awsconfig "github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/config"
	"github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/util"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"k8s.io/utils/ptr"
)

var trustRelationTemplate = `
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Federated": "arn:aws:iam::{{ .Account }}:oidc-provider/oidc.eks.{{ .Region }}.amazonaws.com/id/{{ .OIDCIdentifier }}"
            },
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Condition": {
                "StringEquals": {
                    "oidc.eks.{{ .Region }}.amazonaws.com/id/{{ .OIDCIdentifier }}:aud": "sts.amazonaws.com",
                    "oidc.eks.{{ .Region }}.amazonaws.com/id/{{ .OIDCIdentifier }}:sub": "system:serviceaccount:{{ .SvcNamespace }}:{{ .SvcName }}"
                }
            }
        }
    ]
}`

func updateRoleTrustPolicy(ctx context.Context, cfg aws.Config, clusterConfig awsconfig.ClusterConfiguration, issuerID, roleArn, serviceAccountName, serviceAccountNamespace string) (d diag.Diagnostics) {
	var b bytes.Buffer
	arnParts := strings.Split(roleArn, "/")
	roleName := arnParts[len(arnParts)-1]

	trustRelationTmpl := template.Must(template.New("trustRelation").Parse(trustRelationTemplate))
	if err := trustRelationTmpl.Execute(&b, map[string]any{
		"Account":        clusterConfig.AccountId.ValueString(),
		"Region":         cfg.Region,
		"OIDCIdentifier": issuerID,
		"SvcNamespace":   serviceAccountNamespace,
		"SvcName":        serviceAccountName,
	}); err != nil {
		d.AddError("failed to render trust relation template for role "+roleName, err.Error())
		return
	}

	iamclient := iam.NewFromConfig(cfg)
	if _, err := iamclient.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyDocument: aws.String(strings.TrimSpace(b.String())),
	}); err != nil {
		d.AddError("failed to update role trust relation for role "+roleName, err.Error())
		return
	}

	return
}

func updateRoleTrustPolicies(ctx context.Context, cfg aws.Config, dp awsconfig.AWSDataplane) (d diag.Diagnostics) {
	clusterConfig, diags := dp.ClusterConfigurationData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	cluster, err := util.DescribeKubeCluster(ctx, dp, cfg)
	if err != nil {
		d.AddError("failed to describe EKS cluster", err.Error())
		return
	}

	issArr := strings.Split(ptr.Deref(cluster.Identity.Oidc.Issuer, ""), "/")
	issuerID := issArr[len(issArr)-1]

	d.Append(updateRoleTrustPolicy(ctx, cfg, clusterConfig, issuerID, clusterConfig.DpManagerRoleArn.ValueString(), "dp-manager", "deltastream")...)
	if d.HasError() {
		return
	}

	d.Append(updateRoleTrustPolicy(ctx, cfg, clusterConfig, issuerID, clusterConfig.StoreProxyRoleArn.ValueString(), "store-proxy", "deltastream")...)
	if d.HasError() {
		return
	}

	d.Append(updateRoleTrustPolicy(ctx, cfg, clusterConfig, issuerID, clusterConfig.WorkloadManagerRoleArn.ValueString(), "dp-operator-sa", "dp-operator")...)
	if d.HasError() {
		return
	}

	return
}
