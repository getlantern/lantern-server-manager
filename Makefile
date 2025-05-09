lantern-server-manager:
	CGO_ENABLED=0 go build -ldflags="-extldflags=-static" -o lantern-server-manager ./cmd/...

packer:
	@if [ -z "$(PKR_VAR_aws_secret_key)" ]; then \
		echo "Error: PKR_VAR_aws_secret_key is not set"; \
		exit 1; \
	fi
	@if [ -z "$(PKR_VAR_aws_access_key)" ]; then \
		echo "Error: PKR_VAR_aws_access_key is not set"; \
		exit 1; \
	fi
	@if [ -z "$(PKR_VAR_do_api_token)" ]; then \
		echo "Error: PKR_VAR_do_api_token is not set"; \
		exit 1; \
	fi

	@if [ -z "$(PKR_VAR_gcp_project_id)" ]; then \
		echo "Error: PKR_VAR_gcp_project_id is not set"; \
		exit 1; \
	fi
	@if [ -z "$(PKR_VAR_gcp_zone)" ]; then \
		echo "Error: PKR_VAR_gcp_zone is not set"; \
		exit 1; \
	fi

	# Make sure you have packer installed
	@if ! command -v packer &> /dev/null; then \
		echo "Error: packer is not installed"; \
		exit 1; \
	fi

	cd cloud/packer && packer build .