# ES License Validator

Customer-side license validation service for ES products. This service validates JWT-based licenses, enforces node limits, and reports telemetry back to the ES License Server.

## Overview

The ES License Validator runs in customer Kubernetes clusters to:
- Validate license JWTs using ES's public key
- Enforce node count limits based on labeled nodes
- Handle grace periods for expired licenses
- Phone home to ES License Server (with fail-open behavior)
- Provide health/ready/status endpoints for integration

## Features

- ✅ **JWT Signature Validation** - Verifies licenses signed with RSA-512
- ✅ **Node Count Enforcement** - Counts nodes with `es-products.io/licensed=true` label
- ✅ **Grace Period Handling** - Allows operations during grace period after expiration
- ✅ **Phone Home Telemetry** - Reports validation status to ES License Server (fail-open)
- ✅ **Health Endpoints** - `/health`, `/ready`, `/status` for monitoring
- ✅ **Fail-Open Design** - Continues operation if license server is unreachable
- ✅ **Configurable Intervals** - Customizable validation and phone home frequencies

## Installation

### Prerequisites

1. ES product license (JWT format)
2. Kubernetes cluster with labeled nodes
3. ES public key for JWT verification

### Quick Start

1. **Label your nodes:**
```bash
kubectl label nodes <node-name> es-products.io/licensed=true
```

2. **Create license Secret:**
```bash
kubectl create secret generic es-license \
  --from-literal=license.jwt="<your-license-jwt>"
```

3. **Deploy RBAC:**
```bash
kubectl apply -f deploy/kubernetes/rbac.yaml
```

4. **Deploy validator:**
```bash
kubectl apply -f deploy/kubernetes/deployment.yaml
```

5. **Check status:**
```bash
kubectl logs -l app=es-license-validator -f
```

## Configuration

All configuration is done via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `LICENSE_SECRET_NAME` | `es-license` | Name of Kubernetes Secret containing license |
| `LICENSE_SECRET_NAMESPACE` | `default` | Namespace of license Secret |
| `LICENSE_SECRET_KEY` | `license.jwt` | Key in Secret containing JWT |
| `NODE_LABEL_KEY` | `es-products.io/licensed` | Node label key to count |
| `NODE_LABEL_VALUE` | `true` | Node label value to match |
| `LICENSE_SERVER_URL` | - | ES License Server URL (required if phone home enabled) |
| `PHONE_HOME_ENABLED` | `true` | Enable phone home reporting |
| `PHONE_HOME_INTERVAL` | `24h` | How often to phone home |
| `VALIDATION_INTERVAL` | `5m` | How often to validate license |
| `FAIL_OPEN` | `true` | Allow operations when license server unreachable |
| `HTTP_PORT` | `8080` | HTTP server port |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |

## API Endpoints

### Health Check
```bash
GET /health
```
Returns service health (always returns 200 if service is running).

### Readiness Check
```bash
GET /ready
```
Returns 200 if license is valid (or in grace period with fail-open), 503 otherwise.

### Status
```bash
GET /status
```
Returns detailed license validation status:
```json
{
  "valid": true,
  "validation_time": "2025-10-22T10:00:00Z",
  "node_count": 3,
  "licensed_nodes": 5,
  "days_until_expiry": 25,
  "in_grace_period": false,
  "signature_valid": true,
  "expiry_valid": true,
  "node_count_valid": true,
  "license": {
    "license_id": "...",
    "customer_name": "Acme Corp",
    "product_code": "ES-CORE-GW",
    "product_name": "ES Core Gateway",
    "tier_code": "PROFESSIONAL",
    "cluster_id": "prod-cluster-001",
    "expires_at": "2025-11-16T00:00:00Z"
  }
}
```

## License Validation Logic

1. **Read license JWT** from Kubernetes Secret
2. **Verify JWT signature** using ES public key (RSA-512)
3. **Count labeled nodes** matching `es-products.io/licensed=true`
4. **Check expiration** and grace period
5. **Validate node count** against license limit
6. **Report result** to ES License Server (if phone home enabled)

### Validation States

- **Valid**: All checks pass
- **Grace Period**: Expired but within grace period (operations allowed if fail-open)
- **Invalid**: Failed validation (operations blocked)

## Integration with ES Products

ES products can check validator status before starting:

```bash
# In your product's startup script or init container
curl -f http://es-license-validator/ready || exit 1
```

Or check detailed status:

```bash
STATUS=$(curl -s http://es-license-validator/status)
echo $STATUS | jq '.valid'
```

## Troubleshooting

### Validator pod not starting
```bash
kubectl logs -l app=es-license-validator
kubectl describe pod -l app=es-license-validator
```

### License not found
```bash
kubectl get secret es-license -o yaml
kubectl logs -l app=es-license-validator | grep "license"
```

### Node count mismatch
```bash
kubectl get nodes -L es-products.io/licensed
kubectl logs -l app=es-license-validator | grep "node_count"
```

### Phone home failing
Check connectivity to ES License Server:
```bash
kubectl exec -it deploy/es-license-validator -- wget -O- http://35.224.53.94/health
```

## Development

### Build locally
```bash
go build -o validator ./cmd/validator
```

### Run locally (requires kubeconfig)
```bash
export LICENSE_SECRET_NAME=es-license
export LICENSE_SECRET_NAMESPACE=default
export LICENSE_SERVER_URL=http://35.224.53.94
./validator
```

### Build Docker image
```bash
docker build -t es-license-validator:latest .
```

## License

Proprietary - EnterpriseSight, Inc.

## Support

For issues or questions, contact support@enterprisesight.com
