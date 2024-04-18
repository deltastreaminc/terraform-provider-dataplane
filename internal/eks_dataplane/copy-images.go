// Copyright (c) DeltaStream, Inc.
// SPDX-License-Identifier: Apache-2.0

package eksdataplane

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"sigs.k8s.io/yaml"
)

func CopyImages(ctx context.Context, cfg aws.Config, dp EKSDataplane) (d diag.Diagnostics) {
	clusterConfig, diags := dp.ClusterConfigurationData(ctx)
	d.Append(diags...)
	if d.HasError() {
		return
	}

	bucketName := "prod-ds-packages-maven"
	if clusterConfig.Stack.ValueString() != "prod" {
		bucketName = "deltastream-packages-maven"
	}

	bucketCfg := cfg.Copy()
	bucketCfg.Region = "us-east-2"
	s3client := s3.NewFromConfig(bucketCfg)
	imageListPath := fmt.Sprintf("deltastream-release-images/image-list-%s.yaml", clusterConfig.ProductVersion.ValueString())
	tflog.Debug(ctx, "downloading image list", map[string]any{
		"bucket":          bucketName,
		"image list path": imageListPath,
	})
	getObjectOut, err := s3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(imageListPath),
	})
	if err != nil {
		d.AddError("error getting image list", err.Error())
		return
	}
	defer getObjectOut.Body.Close()

	imageList := struct {
		Images            []string `json:"images"`
		ExecEngineVersion string   `json:"execEngineVersion"`
	}{}

	b, err := io.ReadAll(getObjectOut.Body)
	if err != nil {
		d.AddError("error reading image list", err.Error())
		return
	}
	if err := yaml.Unmarshal(b, &imageList); err != nil {
		d.AddError("error unmarshalling image list", err.Error())
		return
	}

	// Create an Amazon ECR service client
	client := ecr.NewFromConfig(cfg)

	authTokenOut, err := client.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		d.AddError("error getting authorization token", err.Error())
		return
	}

	tokenBytes, err := base64.StdEncoding.DecodeString(*authTokenOut.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		d.AddError("error decoding authorization token", err.Error())
		return
	}
	imageCredContext := &types.SystemContext{
		DockerAuthConfig: &types.DockerAuthConfig{
			Username: "AWS",
			Password: strings.TrimPrefix(string(tokenBytes), "AWS:"),
		},
	}

	for _, image := range imageList.Images {
		sourceImage := fmt.Sprintf("//%s.dkr.ecr.%s.amazonaws.com/%s", clusterConfig.DsAccountId.ValueString(), cfg.Region, image)
		destImage := fmt.Sprintf("//%s.dkr.ecr.%s.amazonaws.com/%s", clusterConfig.AccountId.ValueString(), cfg.Region, image)
		err = copyImage(ctx, imageCredContext, sourceImage, destImage)
		if err != nil {
			d.AddError("error copying image", err.Error())
			return
		}
	}

	execEngineUri := fmt.Sprintf("release/io/deltastream/execution-engine/%s/execution-engine-%s.jar", imageList.ExecEngineVersion, imageList.ExecEngineVersion)
	// Copy the execution engine jar
	tflog.Debug(ctx, "downloading execution engine jar "+bucketName+" "+execEngineUri)
	getObjectOut, err = s3client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(execEngineUri),
	})
	if err != nil {
		d.AddError("error downloading execution engine jar", err.Error())
		return
	}
	defer getObjectOut.Body.Close()
	b, err = io.ReadAll(getObjectOut.Body)
	if err != nil {
		d.AddError("error reading execution engine jar", err.Error())
		return
	}

	tflog.Debug(ctx, "uploading execution engine jar", map[string]any{
		"bucket": clusterConfig.ProductArtifactsBucket.ValueString(),
		"uri":    execEngineUri,
		"size":   len(b),
	})
	uploadS3Client := s3.NewFromConfig(cfg)
	// Upload the execution engine jar to the new bucket
	_, err = uploadS3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(clusterConfig.ProductArtifactsBucket.ValueString()),
		Key:    aws.String(execEngineUri),
		Body:   bytes.NewBuffer(b),
	})
	if err != nil {
		d.AddError("error uploading execution engine jar", err.Error())
		return
	}

	return
}

func copyImage(ctx context.Context, credContext *types.SystemContext, sourceImage, destImage string) (err error) {
	tflog.Debug(ctx, "copying image", map[string]any{
		"source": sourceImage,
		"dest":   destImage,
	})
	srcRef, err := docker.ParseReference(sourceImage)
	if err != nil {
		return fmt.Errorf("error parsing source image: %w", err)
	}

	destRef, err := docker.ParseReference(destImage)
	if err != nil {
		return fmt.Errorf("error parsing destination image: %w", err)
	}

	policy := &signature.Policy{Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()}}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return fmt.Errorf("error creating new policy context: %w", err)
	}

	b := bytes.NewBuffer(nil)
	_, err = copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{
		SourceCtx:      credContext,
		DestinationCtx: credContext,
		ReportWriter:   b,
	})
	if err != nil {
		return fmt.Errorf("error copying image: %w\n%s", err, b.String())
	}

	return
}
