default: download_assets fmt doc testacc
	go install .

.PHONY: fmt
fmt:
	terraform fmt -recursive ./examples/

.PHONY: doc
doc:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate -provider-name dataplane

.PHONY: testacc
testacc:
	TF_ACC=1 go test -v ./... -v $(TESTARGS) -timeout 120m

clean:
	rm -rf docs
	rm internal/deltastream/aws/assets/cilium-*.tgz
	rm internal/deltastream/aws/assets/flux-system/gotk-components.yaml
	rm internal/deltastream/aws/assets/flux-system/flux.yaml.tmpl

download_assets:
	- helm repo add cilium https://helm.cilium.io/
	helm repo update
	cd internal/deltastream/aws/assets && helm pull cilium/cilium --version 1.15.1
	cd internal/deltastream/aws/assets/flux-system && \
		flux install --network-policy=false --export > gotk-components.yaml && \
		kustomize build . > flux.yaml.tmpl
