# Skaffold YAML Reference (local copy)

Source: https://skaffold.dev/docs/references/yaml/
Schema version used in this project: `skaffold/v4beta13`

---

## Top-level structure

```yaml
apiVersion: skaffold/v4beta13   # required
kind: Config                    # required

metadata:
  name: string                  # logical name for this config

dependencies:                   # pull in other skaffold configs
  - configs: [string]
    path: string

build: BuildConfig
test: [TestCase]
render: RenderConfig
manifests: ManifestsConfig      # shorthand for render.generate
deploy: DeployConfig
portForward: [PortForwardResource]
profiles: [Profile]
```

---

## build

Controls how container images are built.

```yaml
build:
  artifacts:           # list of images to build
    - image: string    # required — fully-qualified image name
      context: string  # build context dir (default: .)
      docker:
        dockerfile: string        # path to Dockerfile (default: Dockerfile)
        target: string            # multi-stage target
        buildArgs: {k: v}         # --build-arg
        noCache: bool
        squash: bool
        network: string           # bridge | host | none
      sync:
        manual:
          - src: "**/*.go"
            dest: /app
        infer: [string]
        auto: {}

  local:
    push: bool               # push to registry after build (default: false)
    useBuildkit: bool        # use BuildKit (default: true)
    concurrency: int         # parallel builds (0 = unlimited)
    tryImportMissing: bool

  googleCloudBuild:
    projectID: string
    diskSizeGb: int
    machineType: string      # N1_HIGHCPU_8 etc.
    timeout: string          # e.g. "20m"
    logging: string          # CLOUD_LOGGING_ONLY | GCS_ONLY | LEGACY
    workerPool: string

  cluster:                   # Kaniko in-cluster build
    pullSecretName: string
    namespace: string
    resources:
      requests: {cpu, memory}
      limits: {cpu, memory}

  tagPolicy:
    gitCommit:
      variant: Tags | CommitSha | AbbrevCommitSha | TreeSha | AbbrevTreeSha
      ignoreChanges: bool
    sha256: {}
    envTemplate:
      template: string       # e.g. "{{.IMAGE_TAG}}"
    dateTime:
      format: string         # Go time format
      timezone: string
    customTemplate:
      template: string
      components:
        - name: string
          dateTime: ...
          envTemplate: ...

  platforms: [string]        # e.g. [linux/amd64, linux/arm64]
  insecureRegistries: [string]
```

---

## manifests (shorthand)

Shorthand for `render.generate`. Use when you don't need transforms or validators.

```yaml
manifests:
  rawYaml: [string]          # paths to raw YAML files
  kustomize:
    paths: [string]          # paths to kustomization directories
    buildArgs: [string]      # extra args passed to kustomize build
  helm:
    releases:
      - name: string
        chartPath: string
        remoteChart: string
        repo: string
        version: string
        namespace: string
        createNamespace: bool
        values: {k: v}
        valuesFiles: [string]
        setValues: {k: v}
        setValueTemplates: {k: v}
        wait: bool
        upgradeOnChange: bool
```

---

## render (full form)

```yaml
render:
  generate:
    rawYaml: [string]
    kustomize:
      paths: [string]
      buildArgs: [string]
    helm:
      releases: [HelmRelease]  # same fields as manifests.helm.releases

  transform:
    - name: string             # transformer name
      configMap: [string]

  validate:
    - name: string             # validator name
      configMap: [string]

  output: string               # directory to write hydrated manifests
```

---

## deploy

```yaml
deploy:
  kubectl:
    defaultNamespace: string
    flags:
      global: [string]         # flags for all kubectl commands
      apply: [string]          # extra flags for kubectl apply
      delete: [string]
    remoteManifests: [string]
    hooks:
      before: [HostHook | ContainerHook]
      after:  [HostHook | ContainerHook]

  helm:
    releases: [HelmRelease]    # same as manifests.helm.releases
    flags:
      global: [string]
      install: [string]
      upgrade: [string]

  # Cloud Run deployer (GCP only)
  cloudrun:
    projectID: string
    region: string

  statusCheck: bool                    # wait for rollout to stabilise (default: true)
  statusCheckDeadlineSeconds: int      # timeout for status check (default: 600)
  tolerateFailuresUntilDeadline: bool  # keep retrying until deadline
  kubeContext: string                  # override active kube context
  logs:
    prefix: string                     # container | podAndContainer | auto | none
```

---

## portForward

```yaml
portForward:
  - resourceType: string     # Service | Deployment | Pod | StatefulSet etc.
    resourceName: string     # metadata.name of the resource
    namespace: string
    port: int                # port on the resource
    localPort: int           # local port to bind (default: same as port)
    address: string          # bind address (default: 127.0.0.1)
```

---

## profiles

Profiles override any part of the top-level pipeline for a named environment.

```yaml
profiles:
  - name: string             # required — invoke with skaffold run -p <name>
    activation:              # auto-activate when conditions match
      - kubeContext: string  # regex match against current kube context
        env: string          # KEY=VALUE or KEY=~REGEX
        command: string      # dev | run | debug | render | build | deploy
    requiresAllActivations: bool  # AND (true) vs OR (false, default)
    build: BuildConfig       # completely replaces top-level build if set
    manifests: ManifestsConfig
    render: RenderConfig
    deploy: DeployConfig
    portForward: [PortForwardResource]
    test: [TestCase]
    patches:                 # JSON Patch (RFC 6902) to surgically modify top-level
      - op: add | remove | replace | copy | move | test
        path: string         # JSON pointer, e.g. /build/artifacts/0/docker/dockerfile
        value: any
```

---

## test

```yaml
test:
  - image: string            # image to test (must be in build.artifacts)
    structureTests: [string] # paths to container-structure-test YAML files
    structureTestArgs: [string]
    custom:
      - command: string
        timeoutSeconds: int
        dependencies:
          paths: [string]
          ignore: [string]
```

---

## hooks (HostHook / ContainerHook)

```yaml
# HostHook — runs on the local machine
- host:
    command: [string]       # e.g. ["sh", "-c", "echo deploying"]
    os: [string]            # linux | darwin | windows (filter)
    dir: string             # working directory

# ContainerHook — runs inside a running container
- container:
    command: [string]
    podName: string
    containerName: string
```

---

## Common CLI commands

```bash
# Start dev loop — build, deploy, watch for changes, stream logs
skaffold dev

# One-shot build + deploy
skaffold run

# Build only
skaffold build

# Render manifests only (no deploy)
skaffold render

# Delete deployed resources
skaffold delete

# With a specific profile
skaffold dev -p stage
skaffold run -p prod

# Override kube context
skaffold run -p prod --kube-context=gke_myproject_asia-southeast1_prod

# Tail logs after deploy
skaffold run --tail

# Specify a custom skaffold.yaml location
skaffold dev -f ./deploy/skaffold.yaml

# Stream debug output
skaffold dev -v debug
```

---

## Environment variable substitution in skaffold.yaml

Skaffold supports `$(ENV_VAR)` substitution in most string fields (image tags, build args, etc.). Example from this project's stage kustomization:

```yaml
images:
  - name: ghcr.io/richmondwang/kadto-api
    newTag: "$(IMAGE_TAG)"   # set IMAGE_TAG=abc123 before running skaffold
```

Set before running:
```bash
export IMAGE_TAG=$(git rev-parse --short HEAD)
skaffold run -p stage
```

---

## Schema versions (for reference)

| Schema version | Skaffold version |
|---|---|
| `skaffold/v4beta13` | v2.13+ (current) |
| `skaffold/v4beta12` | v2.12 |
| `skaffold/v4beta11` | v2.11 |
| `skaffold/v2beta29` | v1.39 (legacy stable) |

The schema version in `apiVersion` must match your installed Skaffold version.
Check: `skaffold version`
Upgrade: `brew upgrade skaffold` or download from https://github.com/GoogleContainerTools/skaffold/releases
