# ES License Validator Helm Chart

Official Helm chart for deploying the ES License Validator to Kubernetes clusters.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.0+
- ES product license (JWT format)

## Installation

### Quick Start

```bash
# Add the repository (when published)
helm repo add enterprisesight https://enterprisesight.github.io/charts
helm repo update

# Install the chart
helm install my-validator enterprisesight/es-license-validator \
  --set licenseServer.url=http://your-license-server \
  --create-namespace \
  --namespace es-licensing
```

### Install from GitHub

```bash
# Clone the repository
git clone https://github.com/enterprisesight/es-license-validator.git
cd es-license-validator

# Install the chart
helm install my-validator ./charts/es-license-validator \
  --set licenseServer.url=http://your-license-server \
  --create-namespace \
  --namespace es-licensing
```

## Post-Installation Steps

### 1. Label Your Nodes

Label the nodes that should be counted towards your license:

```bash
kubectl label nodes node-1 es-products.io/licensed=true
kubectl label nodes node-2 es-products.io/licensed=true
```

### 2. Create License Secret

Create a Kubernetes Secret with your license JWT:

```bash
kubectl create secret generic es-license \
  --from-literal=license.jwt="eyJhbGc..." \
  --namespace es-licensing
```

### 3. Verify Installation

Check the validator status:

```bash
kubectl get pods -n es-licensing
kubectl logs -n es-licensing -l app.kubernetes.io/name=es-license-validator
```

## Configuration

The following table lists the configurable parameters of the ES License Validator chart and their default values.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of validator replicas | `1` |
| `image.repository` | Container image repository | `ghcr.io/enterprisesight/es-license-validator` |
| `image.tag` | Container image tag | `v1.0.0` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `serviceAccount.create` | Create service account | `true` |
| `rbac.create` | Create RBAC resources | `true` |
| `license.secretName` | Name of license Secret | `es-license` |
| `license.secretKey` | Key in Secret containing JWT | `license.jwt` |
| `nodeLabeling.key` | Node label key | `es-products.io/licensed` |
| `nodeLabeling.value` | Node label value | `true` |
| `licenseServer.url` | License server URL | `""` |
| `licenseServer.phoneHomeEnabled` | Enable telemetry | `true` |
| `licenseServer.phoneHomeInterval` | Phone home interval | `24h` |
| `validation.interval` | Validation check interval | `5m` |
| `validation.failOpen` | Fail-open mode | `true` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `resources.limits.cpu` | CPU limit | `200m` |
| `resources.limits.memory` | Memory limit | `256Mi` |

### Example Configuration

```yaml
# values-production.yaml
licenseServer:
  url: https://license.enterprisesight.com
  phoneHomeInterval: 12h

validation:
  interval: 10m

resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 500m
    memory: 512Mi

nodeLabeling:
  key: company.com/licensed
  value: "enabled"
```

Install with custom values:

```bash
helm install my-validator ./charts/es-license-validator \
  -f values-production.yaml \
  --namespace es-licensing
```

## Upgrading

```bash
helm upgrade my-validator enterprisesight/es-license-validator \
  --namespace es-licensing
```

## Uninstalling

```bash
helm uninstall my-validator --namespace es-licensing
```

## Troubleshooting

### Validator pod not starting

```bash
kubectl describe pod -n es-licensing -l app.kubernetes.io/name=es-license-validator
kubectl logs -n es-licensing -l app.kubernetes.io/name=es-license-validator
```

### License not found

```bash
kubectl get secret es-license -n es-licensing -o yaml
```

### Node count issues

```bash
kubectl get nodes -L es-products.io/licensed
```

## Support

For issues or questions:
- GitHub Issues: https://github.com/enterprisesight/es-license-validator/issues
- Email: support@enterprisesight.com
