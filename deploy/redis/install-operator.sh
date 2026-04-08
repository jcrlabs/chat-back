#!/usr/bin/env bash
# Install RedKey Operator (InditexTech/redkeyoperator) into the cluster.
# Run once per cluster — not part of app deploy.
set -euo pipefail

OPERATOR_VERSION="${OPERATOR_VERSION:-0.1.0}"
OPERATOR_IMG="ghcr.io/inditextech/redkey-operator:${OPERATOR_VERSION}"
ROBIN_IMG="ghcr.io/inditextech/redkey-robin:${OPERATOR_VERSION}"
NAMESPACE="redkey-system"

echo "==> Creating namespace ${NAMESPACE}"
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

echo "==> Installing RedKey Operator CRDs and controller (${OPERATOR_VERSION})"
kubectl apply -f "https://raw.githubusercontent.com/InditexTech/redkeyoperator/v${OPERATOR_VERSION}/config/crd/bases/redkey.inditex.dev_redkeyclusters.yaml"
kubectl apply -f "https://raw.githubusercontent.com/InditexTech/redkeyoperator/v${OPERATOR_VERSION}/config/manager/manager.yaml"

echo "==> Waiting for operator to be ready"
kubectl rollout status deployment/redkey-operator-controller-manager -n "${NAMESPACE}" --timeout=120s

echo "==> RedKey Operator installed. Deploy the RedkeyCluster with:"
echo "    kubectl apply -f deploy/redis/redkey-cluster.yaml"
