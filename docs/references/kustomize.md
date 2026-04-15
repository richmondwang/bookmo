# Kustomize Reference

Sourced from https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/
Kustomize version used in this project: v5.7.1 (bundled with kubectl v1.35.3)

---

## Overview

A `kustomization.yaml` is a YAML specification for a Kubernetes Resource Model (KRM) object that describes how to generate or transform other KRM objects.

### Basic structure

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - {pathOrUrl}

generators:
  - {pathOrUrl}

transformers:
  - {pathOrUrl}

validators:
  - {pathOrUrl}
```

### Processing order

1. `resources` — load input objects
2. `generators` — add generated objects to the list
3. `transformers` — modify the list (patches, images, labels, etc.)
4. `validators` — check for errors

---

## resources

Specifies which Kubernetes manifests and kustomization directories to include.

```yaml
resources:
  - deployment.yaml           # local file
  - ../../base                # relative path to another kustomization dir
  - https://github.com/...    # remote URL (git)
```

- Each entry must be a path to a **file** or a path/URL to another **kustomization directory**.
- File paths must be relative to the directory containing the kustomization file.
- A file can contain multiple resources separated by `---`.
- **Security restriction (v4+):** Direct file references must be within or below the kustomization root directory. To share a file across overlays, use a `Component` or place it in a base that is referenced as a directory.

---

## patches

Adds or overrides fields on resources. Patches are applied in the order listed.

Two mechanisms are supported:
1. **Strategic Merge Patch** — Kubernetes-native merging
2. **JSON 6902 Patch** — RFC 6902 JSON operations

```yaml
patches:
  # Strategic merge patch from a file, targeting by name
  - path: patches/api-deployment.yaml
    target:
      kind: Deployment
      name: my-app

  # Strategic merge patch inline
  - patch: |-
      apiVersion: apps/v1
      kind: Deployment
      metadata:
        name: my-app
      spec:
        replicas: 3

  # JSON 6902 patch
  - patch: |-
      - op: replace
        path: /spec/replicas
        value: 2
    target:
      kind: Deployment
      name: my-app
```

### Target selector fields

| Field | Description |
|---|---|
| `group` | API group (e.g. `apps`) |
| `version` | API version (e.g. `v1`) |
| `kind` | Resource kind (e.g. `Deployment`) |
| `name` | Resource name — anchored regex |
| `namespace` | Resource namespace — anchored regex |
| `labelSelector` | Kubernetes label selector |
| `annotationSelector` | Kubernetes annotation selector |

A resource must match **all** specified target fields to receive the patch.

### Options

```yaml
patches:
  - path: my-patch.yaml
    options:
      allowNameChange: true   # default false
      allowKindChange: true   # default false
```

---

## images

Modifies container image names, tags, and digests without requiring patches.

```yaml
images:
  - name: nginx               # match this image name
    newTag: 1.8.0             # override the tag
  - name: my-old-app
    newName: my-new-app       # rename the image
    newTag: v2.0
  - name: alpine
    digest: sha256:24a0c4b...  # pin to a digest
```

| Field | Purpose |
|---|---|
| `name` | Image name to match in manifests |
| `newName` | Replace the image name (registry, repo, or both) |
| `newTag` | Replace the tag |
| `digest` | Replace with a digest (takes precedence over `newTag`) |

Kustomize replaces every matching image reference across all resources, including init containers.

### CI/CD pattern

```bash
kustomize edit set image myapp:$(git log -n 1 --pretty=format:"%H")
# or via environment variable
kustomize edit set image myapp:$IMAGE_TAG
```

---

## namespace

Adds or overrides the namespace on all resources. Overrides an existing namespace if already set.

```yaml
namespace: kadto-local
```

---

## labels

Adds labels to resources. Replaces the deprecated `commonLabels` field.

```yaml
labels:
  - pairs:
      app.kubernetes.io/part-of: myapp
      environment: production
    includeSelectors: false       # default false — do NOT add to matchLabels
    includeTemplates: true        # default false — add to pod template metadata
    includeVolumeClaimTemplates: false  # default false — for StatefulSet PVCs
```

### Flags

| Flag | Default | When to use |
|---|---|---|
| `includeSelectors` | `false` | Set `true` only for new resources — changing selectors on live Deployments is a breaking change |
| `includeTemplates` | `false` | Set `true` to label pod templates (so pods carry the label) |
| `includeVolumeClaimTemplates` | `false` | Set `true` for StatefulSet PVC templates |

### Migration from `commonLabels`

`commonLabels` is deprecated. It defaults `includeSelectors: true` which is dangerous for existing clusters.

```yaml
# Before (deprecated)
commonLabels:
  app.kubernetes.io/part-of: myapp

# After (safe replacement)
labels:
  - pairs:
      app.kubernetes.io/part-of: myapp
    includeSelectors: false
    includeTemplates: true
```

---

## commonAnnotations

Adds annotations to all resources. Overrides existing annotation values when keys match.

```yaml
commonAnnotations:
  oncallPager: 800-555-1212
  team: platform
```

---

## configMapGenerator

Generates ConfigMap resources. Each entry generates one ConfigMap.

```yaml
configMapGenerator:
  # From files (filename = key)
  - name: app-config
    files:
      - application.properties
      - custom-key=other-file.ini   # custom key name

  # From literals
  - name: env-vars
    literals:
      - JAVA_HOME=/opt/java/jdk
      - LOG_LEVEL=info

  # From .env file
  - name: tracing-options
    envs:
      - tracing.env

  # With options
  - name: my-config
    literals:
      - KEY=value
    options:
      disableNameSuffixHash: true   # disable content hash suffix
      labels:
        app: myapp
      annotations:
        note: generated
```

### Behavior in overlays

| Value | Effect |
|---|---|
| `create` (default) | Create new ConfigMap; error if one exists |
| `replace` | Overwrite existing ConfigMap |
| `merge` | Update values in existing ConfigMap |

```yaml
configMapGenerator:
  - name: existing-config
    behavior: merge
    literals:
      - NEW_KEY=new-value
```

---

## secretGenerator

Generates Secret resources. Functions identically to `configMapGenerator`.

```yaml
secretGenerator:
  - name: app-tls
    files:
      - tls.crt
      - tls.key
    type: kubernetes.io/tls

  - name: db-creds
    literals:
      - DB_PASSWORD=secret
    type: Opaque
    options:
      disableNameSuffixHash: true

  - name: namespaced-secret
    namespace: apps
    files:
      - tls.crt=certs/tls.crt
    type: kubernetes.io/tls
```

> **Note:** Generated secret values are base64-encoded in the output manifest.

---

## components

Allows modular, reusable configuration pieces included by some overlays but not others. A Component uses `kind: Component` and `apiVersion: kustomize.config.k8s.io/v1alpha1`.

### Component definition

```yaml
# components/my-feature/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component

resources:
  - job.yaml
  - configmap.yaml

patches:
  - path: patch.yaml
    target:
      kind: Deployment
      name: my-app
```

### Referencing a component from an overlay

```yaml
# overlays/prod/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

components:
  - ../../components/my-feature   # included in prod
  # local overlay omits this component
```

### When to use components vs bases

| Use case | Approach |
|---|---|
| Resources shared by ALL overlays | Put in `base`, reference in `resources` |
| Resources shared by SOME overlays | Put in a `Component`, reference in `components` |
| Per-overlay configuration override | Use `patches` in the overlay |

Components solve the cross-directory security restriction: a component is a proper kustomization directory, so referencing it is always allowed.

---

## namePrefix / nameSuffix

Prepends or appends a string to the names of all resources and updates all cross-references.

```yaml
namePrefix: prod-
nameSuffix: -v2
```

Propagates automatically to:
- ConfigMap references in PodSpecs
- Secret references in PodSpecs
- Service references from StatefulSets

---

## replacements

Copies a field value from one source resource into any number of target resources.

```yaml
replacements:
  # Inline style
  - source:
      kind: ConfigMap
      name: my-config
      fieldPath: data.HOSTNAME
    targets:
      - select:
          kind: Deployment
          name: my-app
        fieldPaths:
          - spec.template.spec.containers.[name=api].env.[name=HOSTNAME].value

  # File reference style
  - path: replacements/my-replacement.yaml
```

### Source fields

| Field | Description |
|---|---|
| `group/version/kind/name/namespace` | Identify the source resource |
| `fieldPath` | Dot-separated path to the value (supports array indexing) |

### Target fields

| Field | Description |
|---|---|
| `select` | GVKNN or label/annotation selector for target resources |
| `reject` | Exclude resources from selection |
| `fieldPaths` | List of destination paths |
| `options.create` | Create the field if it doesn't exist |
| `options.delimiter` | Split the field value and replace a specific part |

### Field path syntax

```
spec.template.spec.containers.[name=api].env.[name=DB_HOST].value
metadata.annotations.config\.kubernetes\.io/local-config
```

---

## Common patterns in this project

### Overlay overriding a base ConfigMap

The overlay's `configmap.yaml` is a **patch** (not a resource), because the base already defines the ConfigMap. Adding it as a resource would create a duplicate ID error.

```yaml
# overlays/local/kustomization.yaml
patches:
  - path: configmap.yaml
    target:
      kind: ConfigMap
      name: kadto-config
```

### Migration Job in stage/prod only

The Job lives in `deploy/components/migrate/` as a Component. Local overlay omits the component; stage and prod include it:

```yaml
# overlays/stage/kustomization.yaml
components:
  - ../../components/migrate
```

This avoids the kustomize security restriction that blocks direct file references outside the overlay directory (`../../base/migrate-job.yaml` would be rejected).
