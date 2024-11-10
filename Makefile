# Docker image to run shell and go utility functions in
WORKER_IMAGE = golang:1.23.2-alpine3.20
# Docker image to generate OAS3 specs
OAS3_GENERATOR_DOCKER_IMAGE = openapitools/openapi-generator-cli:latest-release

MKDOCS_IMAGE = jamshi/mk-docs-gh:latest

.PHONY: ci-swaggen2
ci-swaggen2:
	@docker run --rm -v $(PWD):/work $(WORKER_IMAGE) \	  sh -c "apk update && apk add --no-cache git && export PATH=$PATH:$(go env GOPATH)/bin && go install github.com/swaggo/swag/cmd/swag@latest && cd /work && swag init -g ./rest-api/routes.go"

# Generate OAS3 from swaggo/swag output since that project doesn't support it
# TODO: Remove this if V3 spec is ever returned from that project
.PHONY: ci-swaggen
ci-swaggen: ci-swaggen2
	@echo "[OAS3] Converting Swagger 2-to-3 (yaml)"
	@docker run --rm -v $(PWD)/docs:/work $(OAS3_GENERATOR_DOCKER_IMAGE) \
	  generate -i /work/swagger.yaml -o /work/v3 -g openapi-yaml --minimal-update
	@docker run --rm -v $(PWD)/docs/v3:/work $(WORKER_IMAGE) \
	  sh -c "rm -rf /work/.openapi-generator"
	@echo "[OAS3] Copying openapi-generator-ignore (json)"
	@docker run --rm -v $(PWD)/docs/v3:/work $(WORKER_IMAGE) \
	  sh -c "cp -f /work/.openapi-generator-ignore /work/openapi"
	@echo "[OAS3] Converting Swagger 2-to-3 (json)"
	@docker run --rm -v $(PWD)/docs:/work $(OAS3_GENERATOR_DOCKER_IMAGE) \
	  generate -s -i /work/swagger.json -o /work/v3/openapi -g openapi --minimal-update
	@echo "[OAS3] Cleaning up generated files"
	@docker run --rm -v $(PWD)/docs/v3:/work $(WORKER_IMAGE) \
	  sh -c "mv -f /work/openapi/openapi.json /work ; mv -f /work/openapi/openapi.yaml /work ; rm -rf /work/openapi"
	@echo "Fine tuning openapi.json..."
	@docker run --rm -v $(PWD):/work $(WORKER_IMAGE) \
			sh -c "wget https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64 && \
			mv jq-linux64 /usr/local/bin/jq && \
			chmod +x /usr/local/bin/jq; \
		apk add bash; cd /work/; .devops/scripts/update_openapi.sh"
	@docker run --rm -v $(PWD):/work $(MKDOCS_IMAGE) \
	  gh-deploy

.PHONY: gh-deploy
gh-deploy: ci-swaggen2
	@docker run --rm -v $(PWD):/work -v ${HOME}/.config:/root/.config -v ${HOME}/.gitconfig-copy:/root/.gitconfig \
	$(MKDOCS_IMAGE) \
	bash -c "cd /work;mkdocs gh-deploy"

.PHONY: gen-ecs256-pair
gen-ecs256-pair:
	@openssl ecparam -genkey -name prime256v1 -noout -out ec_private.pem && openssl ec -in ec_private.pem -pubout -out ec_public.pem

.PHONY: migration-generate
migration-generate:
	atlas migrate diff --env gorm

.PHONY: migration-apply
migration-apply:
	atlas migrate apply --env gorm --url "postgres://bigbucks:bigbucks@localhost:5432/bigbucks?search_path=public&sslmode=disable"