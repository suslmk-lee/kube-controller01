#!/bin/bash

# KEBE Controller Cleanup Script
# This script removes all deployed KEBE controller resources from the Kubernetes cluster

set -e

echo "🧹 Starting KEBE Controller cleanup..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to check if kubectl is available
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        echo -e "${RED}❌ kubectl is not installed or not in PATH${NC}"
        exit 1
    fi
}

# Function to check if namespace exists
check_namespace() {
    if ! kubectl get namespace k-paas-system &> /dev/null; then
        echo -e "${YELLOW}⚠️  Namespace 'k-paas-system' does not exist. Nothing to clean up.${NC}"
        exit 0
    fi
}

# Function to delete resources with error handling
delete_resource() {
    local resource_type=$1
    local resource_name=$2
    local namespace=$3
    
    if [ -n "$namespace" ]; then
        local ns_flag="-n $namespace"
    else
        local ns_flag=""
    fi
    
    if kubectl get $resource_type $resource_name $ns_flag &> /dev/null; then
        echo "  Deleting $resource_type/$resource_name..."
        kubectl delete $resource_type $resource_name $ns_flag --ignore-not-found=true
    else
        echo "  $resource_type/$resource_name not found, skipping..."
    fi
}

# Main cleanup function
main() {
    check_kubectl
    check_namespace
    
    echo -e "${YELLOW}📋 Cleaning up KEBE Controller resources...${NC}"
    
    # Delete test LoadBalancer service if exists
    echo "🔍 Checking for test LoadBalancer service..."
    delete_resource "service" "test-loadbalancer" "default"
    
    # Delete controller deployment
    echo "🚀 Removing controller deployment..."
    delete_resource "deployment" "controller-manager" "k-paas-system"
    
    # Delete service account
    echo "👤 Removing service account..."
    delete_resource "serviceaccount" "controller-manager" "k-paas-system"
    
    # Delete cluster role binding
    echo "🔐 Removing cluster role binding..."
    delete_resource "clusterrolebinding" "manager-rolebinding" ""
    
    # Delete cluster role
    echo "📜 Removing cluster role..."
    delete_resource "clusterrole" "manager-role" ""
    
    # Delete secret
    echo "🔑 Removing credentials secret..."
    delete_resource "secret" "naver-cloud-credentials" "k-paas-system"
    
    # Delete namespace (ask for confirmation)
    echo ""
    read -p "🗑️  Do you want to delete the 'k-paas-system' namespace? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "🗂️  Deleting namespace k-paas-system..."
        kubectl delete namespace k-paas-system --ignore-not-found=true
        echo -e "${GREEN}✅ Namespace deleted${NC}"
    else
        echo -e "${YELLOW}⚠️  Keeping namespace 'k-paas-system'${NC}"
    fi
    
    echo ""
    echo -e "${GREEN}🎉 KEBE Controller cleanup completed!${NC}"
    echo ""
    echo "📝 Summary of cleaned up resources:"
    echo "  - Deployment: controller-manager"
    echo "  - ServiceAccount: controller-manager"
    echo "  - ClusterRole: manager-role"
    echo "  - ClusterRoleBinding: manager-rolebinding"
    echo "  - Secret: naver-cloud-credentials"
    echo "  - Test Service: test-loadbalancer (if existed)"
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "  - Namespace: k-paas-system"
    fi
    echo ""
    echo -e "${YELLOW}💡 To redeploy, run: ./quick-deploy.sh${NC}"
}

# Run main function
main "$@"