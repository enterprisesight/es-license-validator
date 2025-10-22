# Future Enhancements for ES License Validator

## Phase 2: Per-Product Licensing (Priority: HIGH)

### Current Limitation (v1.0.0)
The current implementation uses a **single cluster-wide validator** that:
- Counts all nodes with label `es-products.io/licensed=true`
- Validates one license for the entire cluster
- All ES products share the same node license pool

### Future Requirement
Support **per-product, per-node licensing** where:
- Each ES product (ES-Core-GW, ES-DAG-Builder, etc.) has its own license
- Each product can have different node limits
- Products can run on overlapping or separate node sets
- Customer pays per product, per node

### Implementation Options

#### Option A: Per-Namespace Validator Instances
Deploy a validator instance in each product namespace:

```yaml
# es-core-gw namespace
apiVersion: v1
kind: Secret
metadata:
  name: es-license
  namespace: es-core-gw
data:
  license.jwt: <ES-Core-GW license - 2 nodes>

---
# Validator deployment in es-core-gw namespace
# Validates only ES-Core-GW license
```

**Pros:**
- Clean separation per product
- Different node limits per product
- Independent validation cycles

**Cons:**
- Multiple validator pods (resource overhead)
- More complex deployment

#### Option B: Single Validator with Multi-License Support
One validator instance that validates multiple product licenses:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: es-licenses
  namespace: default
data:
  es-core-gw.jwt: <license for 2 nodes>
  es-dag-builder.jwt: <license for 3 nodes>
  es-agent-builder.jwt: <license for 1 node>
```

Validator reads all licenses and validates node counts per product based on namespace labels.

**Pros:**
- Single validator pod
- Centralized licensing view
- Lower resource usage

**Cons:**
- More complex validator logic
- Single point of failure for all products

#### Option C: Hybrid Approach with Node Pools
Use Kubernetes node pools with product-specific labels:

```yaml
# Node pool 1: Licensed for ES-Core-GW
es-products.io/es-core-gw: "true"

# Node pool 2: Licensed for ES-DAG-Builder
es-products.io/es-dag-builder: "true"

# Node pool 3: Licensed for both
es-products.io/es-core-gw: "true"
es-products.io/es-dag-builder: "true"
```

**Pros:**
- Flexible overlapping licenses
- Clear node assignment per product
- Accurate billing per product

**Cons:**
- Complex labeling strategy
- Validator needs to count nodes per product label

### Recommended Approach

**Phase 2.1:** Implement Option B (Multi-License Support)
- Easier migration from current single-license model
- Provides per-product licensing without multiple validators
- Customer-friendly: one validator installation

**Phase 2.2:** Add Option C (Product-Specific Node Labels)
- Enable overlapping node pools
- More granular control for customers
- Better reporting for ES: "Customer X uses 5 nodes for GW, 3 for DAG Builder"

### Data Model Changes

**Current License JWT:**
```json
{
  "license_id": "...",
  "product_code": "ES-CORE-GW",
  "licensed_nodes": 2
}
```

**Future Multi-Product License:**
```json
{
  "license_id": "...",
  "products": [
    {
      "product_code": "ES-CORE-GW",
      "licensed_nodes": 2,
      "node_selector": "es-products.io/es-core-gw=true"
    },
    {
      "product_code": "ES-DAG-Builder",
      "licensed_nodes": 3,
      "node_selector": "es-products.io/es-dag-builder=true"
    }
  ]
}
```

### Validator Changes Required

1. **Multi-license reading** - Read multiple JWT files from Secret
2. **Per-product node counting** - Count nodes per product label
3. **Per-product validation** - Validate each product independently
4. **Status endpoint updates** - Return status for all products
5. **Phone home updates** - Report per-product usage

### Billing Implications

**Current (v1.0):**
- Customer pays for: 1 cluster license × N nodes
- Example: 5 nodes = 1 license

**Future (v2.0):**
- Customer pays for: P products × N nodes (per product)
- Example:
  - ES-Core-GW on 3 nodes = License 1 (3 nodes)
  - ES-DAG-Builder on 2 nodes = License 2 (2 nodes)
  - Total: 2 licenses, potential node overlap

### Migration Path

1. **v1.0.0** (Current) - Single cluster-wide license
2. **v1.1.0** - Add support for reading multiple licenses (backwards compatible)
3. **v1.2.0** - Add per-product node labeling and validation
4. **v2.0.0** - Default to per-product licensing model

### Documentation Updates Needed

- Update customer installation guide with per-product examples
- Create billing documentation for multi-product scenarios
- Add troubleshooting guide for overlapping node pools
- Document migration from v1.x to v2.x

---

**Note:** This design was identified on 2025-10-22 during initial implementation.
The current v1.0.0 implementation intentionally uses the simpler single-license
model to get the licensing infrastructure working end-to-end. Per-product licensing
is the next priority after v1.0.0 validation.
