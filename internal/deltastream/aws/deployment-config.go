package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"k8s.io/utils/ptr"

	awsconfig "github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/config"
	"github.com/deltastreaminc/terraform-provider-dataplane/internal/deltastream/aws/util"
)

const deploymentConfigTmpl = `
{
  "vault": {
    "kms": {
      "key_id": "{{ .KmsKeyId }}",
      "region": "{{ .Region }}"
    },
    "dynamodb": {
      "table": "{{ .DynamoDbTable }}",
      "region": "{{ .Region }}"
    }
  },
  "postgres": {
    "username": "{{ .Rds.Username }}",
    "password": "{{ .Rds.Password }}",
    "database": "{{ .Rds.Database }}",
    "sslMode": "require",
    "host": "{{ .Rds.Host }}",
    "port": {{ .Rds.Port }}
  },
  "kafka": {
    "hosts": "{{ .KafkaBrokerList }}",
    "bootstrapBrokersIam": "{{ .KafkaBrokerList }}",
    "brokerListenerPorts": "{{ .KafkaBrokerListenerPorts }}",
    "enableTLS": true,
    "topicReplicas": 3,
    "region": "{{ .Region }}",
    "roleARN": "{{ .KafkaRoleARN }}",
    "externalId": "{{ .KafkaRoleExternalId }}"
  },
  "cpKafka": {
    "hosts": "{{ .ControlPlaneKafkaBrokerList }}",
    "bootstrapBrokersIam": "{{ .ControlPlaneKafkaBrokerList }}",
    "brokerListenerPorts": "{{ .ControlPlaneKafkaBrokerListenerPorts }}",
    "topicReplicas": 3,
    "region": "{{ .ControlPlaneRegion }}"
  },
  "hostnames": {
    "dpAPIHostname": "{{ .ApiHostname }}"
  },
  "googleOAuth": {
    "clientID": "{{ .DSSecret.GoogleClientID }}",
    "clientSecret": "{{ .DSSecret.GoogleClientSecret }}"
  },
  "s3": {
    "execEngineBucket": {
      "name": "{{ .ProductArtifactsBucket }}",
      "region": "{{ .Region }}"
    },
    "serdeDescriptorBucket": {
      "name": "{{ .SerdeBucket }}",
      "region": "{{ .SerdeBucketRegion }}"
    },
    "flinkQueryStateBucket": {
      "name": "{{ .WorkloadStateBucket }}",
      "region": "{{ .Region }}"
    },
    "lokiRulerStorageBucket": {
      "name": "{{ .O11yBucket }}",
      "region": "{{ .Region }}"
    },
    "lokiStorageBucket": {
      "name": "{{ .O11yBucket }}",
      "region": "{{ .Region }}"
    },
    "lokiAdminBucket": {
      "name": "{{ .O11yBucket }}",
      "region": "{{ .Region }}"
    },
    "prometheusStorageBucket": {
      "name": "{{ .O11yBucket }}",
      "region": "{{ .Region }}"
    },
    "tempoStorageBucket": {
      "name": "{{ .O11yBucket }}",
      "region": "{{ .Region }}"
    },
    "cw2loki": {
      "name": "{{ .O11yBucket }}",
      "region": "{{ .Region }}"
    }
  },
  "kube": {
    "storageClass": "gp3"
  },
  "slack": {
    "token": "{{ .DSSecret.SlackToken }}",
    "channel": "{{ .DSSecret.SlackChannel }}",
    "pingUser": "{{ .DSSecret.SlackPingUser }}"
  },
  "pagerduty": {
    "serviceKey": "{{ .DSSecret.PagerdutyServiceKey }}"
  },
  "cw2loki": {
    "eksClusterName": "{{ .KubeClusterName }}",
    "mskClusterName": "{{ .KafkaClusterName }}",
    "rdsName": "{{ .RdsClusterName}}",
    "importBucketAccount": "{{ .AccountID }}",
    "sqsURL": "{{ .Cw2LokiSqsURL }}"
  }
}`

type DSSecrets struct {
	GoogleClientID      string `json:"googleClientID"`
	GoogleClientSecret  string `json:"googleClientSecret"`
	SlackToken          string `json:"slackToken"`
	SlackChannel        string `json:"slackChannel"`
	SlackPingUser       string `json:"slackPingUser"`
	PagerdutyServiceKey string `json:"pagerdutyServiceKey"`
}

type PostgresCredSecret struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"dbClusterIdentifier"`
}

func UpdateDeploymentConfig(ctx context.Context, cfg aws.Config, dp awsconfig.AWSDataplane) (diags diag.Diagnostics) {
	config, dg := dp.ClusterConfigurationData(ctx)
	diags.Append(dg...)
	if diags.HasError() {
		return
	}

	// Get DeltaStream secret with credentials for PagerDuty, Slack, and Google OAuth
	dsCfg := cfg.Copy()
	dsCfg.Region = config.DsRegion.ValueString()
	dsSecretsmanagerClient := secretsmanager.NewFromConfig(dsCfg)
	providerSecretArn := fmt.Sprintf("%s:secret:deltastream/%s/dp/aws/%s/deployment/%s/provider-dataplane", util.GetARNForCPService(ctx, cfg, config, "secretsmanager"), config.Stack.ValueString(), cfg.Region, config.InfraId.ValueString())
	dsSecret, err := dsSecretsmanagerClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: ptr.To(providerSecretArn),
	})
	if err != nil {
		diags.AddError("unable to read DeltaStream secret "+providerSecretArn, err.Error())
		return
	}

	dsSecrets := &DSSecrets{}
	if err := json.Unmarshal([]byte(ptr.Deref(dsSecret.SecretString, string(dsSecret.SecretBinary))), dsSecrets); err != nil {
		diags.AddError("unable to unmarshal DeltaStream secret", err.Error())
		return
	}

	// Get Postgres credentials
	secretsmanagerClient := secretsmanager.NewFromConfig(cfg)
	rdsSecretArn := fmt.Sprintf("%s:secret:deltastream/%s/dp-%s/rds/%s/%s/db/credential-0", util.GetARNForService(ctx, cfg, config, "secretsmanager"), config.Stack.ValueString(), config.InfraId.ValueString(), cfg.Region, config.RdsResourceID.ValueString())
	rdsCred, err := secretsmanagerClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: ptr.To(rdsSecretArn),
	})
	if err != nil {
		diags.AddError("unable to read rds credentials "+rdsSecretArn, err.Error())
		return
	}

	pgCred := &PostgresCredSecret{}
	if err := json.Unmarshal([]byte(ptr.Deref(rdsCred.SecretString, string(rdsCred.SecretBinary))), pgCred); err != nil {
		diags.AddError("unable to unmarshal rds credentials", err.Error())
		return
	}
	if strings.Contains(pgCred.Host, ":") {
		hostPort := strings.Split(pgCred.Host, ":")
		pgCred.Host = hostPort[0]
	}

	tmpl, err := template.New("deploymentConfig").Parse(deploymentConfigTmpl)
	if err != nil {
		diags.AddError("unable to parse deployment config template", err.Error())
		return
	}

	kafkaBrokers := []string{}
	diags.Append(config.KafkaHosts.ElementsAs(ctx, &kafkaBrokers, false)...)
	if diags.HasError() {
		return
	}

	kafkaListenerPorts := []string{}
	diags.Append(config.KafkaListenerPorts.ElementsAs(ctx, &kafkaListenerPorts, false)...)
	if diags.HasError() {
		return
	}

	cpKafkaBrokers := []string{}
	diags.Append(config.ControlPlaneKafkaHosts.ElementsAs(ctx, &cpKafkaBrokers, false)...)
	if diags.HasError() {
		return
	}

	cpKafkaListenerPorts := []string{}
	diags.Append(config.ControlPlaneKafkaListenerPorts.ElementsAs(ctx, &cpKafkaListenerPorts, false)...)
	if diags.HasError() {
		return
	}

	kubeClusterName, diags := util.GetKubeClusterName(ctx, dp)
	diags = append(diags, diags...)
	if diags.HasError() {
		return
	}

	rdsClusterName := fmt.Sprintf("dp-%s-%s-%s-db-0", config.InfraId.ValueString(), config.Stack.ValueString(), config.RdsResourceID.ValueString())
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]any{
		"AccountID":                            config.AccountId.ValueString(),
		"Region":                               cfg.Region,
		"KmsKeyId":                             config.KmsKeyId.ValueString(),
		"DynamoDbTable":                        config.DynamoDbTableName.ValueString(),
		"Rds":                                  pgCred,
		"DSSecret":                             dsSecrets,
		"KafkaBrokerList":                      strings.Join(kafkaBrokers, ","),
		"KafkaBrokerListenerPorts":             strings.Join(kafkaListenerPorts, ","),
		"KafkaRoleARN":                         config.KafkaRoleArn.ValueString(),
		"KafkaRoleExternalId":                  config.KafkaRoleExternalId.ValueString(),
		"ControlPlaneKafkaBrokerList":          strings.Join(cpKafkaBrokers, ","),
		"ControlPlaneKafkaBrokerListenerPorts": strings.Join(cpKafkaListenerPorts, ","),
		"ControlPlaneRegion":                   config.DsRegion.ValueString(),
		"ApiHostname":                          config.ApiHostname.ValueString(),
		"ProductArtifactsBucket":               config.ProductArtifactsBucket.ValueString(),
		"SerdeBucket":                          config.SerdeBucket.ValueString(),
		"SerdeBucketRegion":                    config.DsRegion.ValueString(),
		"WorkloadStateBucket":                  config.WorkloadStateBucket.ValueString(),
		"O11yBucket":                           config.O11yBucket.ValueString(),
		"KubeClusterName":                      kubeClusterName,
		"KafkaClusterName":                     config.KafkaClusterName.ValueString(),
		"RdsClusterName":                       rdsClusterName,
		"Cw2LokiSqsURL":                        config.Cw2LokiSqsUrl.ValueString(),
	})
	if err != nil {
		diags.AddError("unable to render deployment config", err.Error())
		return
	}

	deploymentConfigSecretName := calcDeploymentConfigSecretName(config, cfg.Region)
	if _, err := secretsmanagerClient.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(deploymentConfigSecretName),
	}); err != nil {
		var resourceNotFoundException *types.ResourceNotFoundException
		if errors.As(err, &resourceNotFoundException) {
			if _, err = secretsmanagerClient.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
				Name:         ptr.To(deploymentConfigSecretName),
				SecretString: ptr.To(buf.String()),
				Tags: []types.Tag{
					{Key: ptr.To("deltastream-io-region"), Value: ptr.To(cfg.Region)},
					{Key: ptr.To("deltastream-io-team"), Value: ptr.To("true")},
					{Key: ptr.To("deltastream-io-is-prod"), Value: ptr.To("true")},
					{Key: ptr.To("deltastream-io-env"), Value: ptr.To(config.Stack.ValueString())},
					{Key: ptr.To("deltastream-io-id"), Value: ptr.To(config.InfraId.ValueString())},
					{Key: ptr.To("deltastream-io-name"), Value: ptr.To("dp-" + config.InfraId.ValueString())},
					{Key: ptr.To("deltastream-io-is-byoc"), Value: ptr.To("true")},
				},
			}); err != nil {
				diags.AddError("unable to create deployment config "+deploymentConfigSecretName, err.Error())
				return
			}
		} else {
			diags.AddError("unable to describe deployment config "+deploymentConfigSecretName, err.Error())
			return
		}
	} else {
		if _, err = secretsmanagerClient.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
			SecretId:     ptr.To(deploymentConfigSecretName),
			SecretString: ptr.To(buf.String()),
		}); err != nil {
			diags.AddError("unable to write deployment config "+deploymentConfigSecretName, err.Error())
			return
		}
	}

	return
}

func calcDeploymentConfigSecretName(config awsconfig.ClusterConfiguration, region string) string {
	return fmt.Sprintf("deltastream/%s/dp/%s/aws/%s/%s/deployment-config", config.Stack.ValueString(), config.InfraId.ValueString(), region, config.EksResourceId.ValueString())
}
