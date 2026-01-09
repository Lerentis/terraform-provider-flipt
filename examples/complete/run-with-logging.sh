#!/bin/bash
# Script to run Terraform with verbose logging

# Enable Terraform logging
export TF_LOG=DEBUG
export TF_LOG_PATH=terraform-debug.log

# Also enable Go SDK logging if available
export FLIPT_LOG_LEVEL=debug

echo "Running Terraform with DEBUG logging enabled..."
echo "Logs will be written to: terraform-debug.log"
echo ""

# Run terraform
terraform "$@"

echo ""
echo "Check terraform-debug.log for detailed request/response information"
