resource "dataplane_aws" "deltastream" {
  assume_role = {
    region   = "us-west-2"
    role_arn = "arn:aws:iam::123456789012:role/deltastream-infra-role"
  }

  cluster_configuration = {
    account_id    = "123456789012" # customer account id
    ds_account_id = "345678901234" # DeltaStream account ID

    product_artifacts_bucket = "s3-artifacts-buckets"
    product_version          = "1.0.0" # product version

    infra_id        = "ds123" # infra id (provided by DeltaStream)
    cluster_index   = "0"     # cluster index (provided by DeltaStream)
    eks_resource_id = "0"     # resource id (provided by DeltaStream)

    karpenter_role_name     = "arn:aws:iam::123456789012:role/karpenter-iam-role"
    karpenter_irsa_role_arn = "arn:aws:iam::123456789012:role/karpenter-irsa-role"
    interruption_queue_name = "ds123-0-0-interruption-queue"

    api_hostname    = "api.ds123.com"
    api_subnet_mode = "public"
    api_tls_mode    = "acme"

    o11y_hostname    = "o11y.ds123.com"
    o11y_subnet_mode = "public"
    o11y_tls_mode    = "acme"

    console_hostname = "console.deltastream.io"

    metrics_url            = "https://metrics-push-proxy.deltastream.io"
    dp_manager_cp_role_arn = "arn:aws:iam::123456789012:role/dp-manager-cp-role"

    # deployment roles
    aws_secrets_manager_ro_role_arn = "arn:aws:iam::123456789012:role/aws-secrets-ro-role"
    cw2loki_role_arn                = "arn:aws:iam::123456789012:role/cw2loki-role"
    deadman_alert_role_arn          = "arn:aws:iam::123456789012:role/deadman-alert-role"
    dp_manager_role_arn             = "arn:aws:iam::123456789012:role/dp-manager-role"
    ds_cross_account_role_arn       = "arn:aws:iam::123456789012:role/ds-cross-account-role"
    ecr_readonly_role_arn           = "arn:aws:iam::123456789012:role/ecr-readonly-role"
    infra_manager_role_arn          = "arn:aws:iam::123456789012:role/infra-manager-role"
    store_proxy_role_arn            = "arn:aws:iam::123456789012:role/store-proxy-role"
    loki_role_arn                   = "arn:aws:iam::123456789012:role/loki-role"
    tempo_role_arn                  = "arn:aws:iam::123456789012:role/tempo-role"
    thanos_sidecar_role_arn         = "arn:aws:iam::123456789012:role/thanos-sidecar-role"
    thanos_store_bucket_role_arn    = "arn:aws:iam::123456789012:role/thanos-store-bucket-role"
    thanos_store_compactor_role_arn = "arn:aws:iam::123456789012:role/thanos-store-compactor-role"
    thanos_store_gateway_role_arn   = "arn:aws:iam::123456789012:role/thanos-store-gateway-role"
    vault_init_role_arn             = "arn:aws:iam::123456789012:role/vault-init-role"
    vault_role_arn                  = "arn:aws:iam::123456789012:role/vault-role"
    workload_iam_role_arn           = "arn:aws:iam::123456789012:role/workload-iam-role"
    workload_manager_iam_role_arn   = "arn:aws:iam::123456789012:role/workload-manager-iam-role"

    # vpc info
    vpc_cidr             = "10.0.0.0/16"
    vpc_dns_ip           = "10.21.0.2"
    vpc_id               = "vpc-12345678901234567"
    public_subnet_ids    = ["subnet-12345678901234567", "subnet-12345678901234568"] # public subnet ids for eks
    private_subnet_ids   = ["subnet-12345678901234567", "subnet-12345678901234568"] # private subnet ids for eks
    private_link_subnets = ["subnet-12345678901234567", "subnet-12345678901234568"] # private subnet ids for private link

    workload_credentials_mode = "iamrole"
    workload_role_arn         = "arn:aws:iam::123456789012:role/workload-iam-role"
    workload_manager_role_arn = "arn:aws:iam::123456789012:role/workload-manager-iam-role"
    rds_ca_certs_secret       = "deltastream/rds/ca/rds-certs-bundle"
    retry_install             = "first_retry"
  }
}


