# Provider Infisical

`provider-infisical` is a [Crossplane](https://crossplane.io/) provider that
is built using [Upjet](https://github.com/crossplane/upjet) code
generation tools and exposes XRM-conformant managed resources for the
Infisical API.

## Getting Started

Install the provider by using the following command after changing the image tag
to the [latest release](https://marketplace.upbound.io/providers/infisical/provider-infisical):

```
up ctp provider install xpkg.upbound.io/infisical-inc/provider-infisical:v0.1.4
```

Alternatively, you can use declarative installation:

```
cat <<EOF | kubectl apply -f -
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-infisical
spec:
  package: xpkg.upbound.io/infisical-inc/provider-infisical:v0.1.4
EOF
```

### 🔐 Configuring Credentials

Before using the provider, create a Kubernetes `Secret` that contains your Infisical API credentials.

#### 1. Create a Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: example-creds
  namespace: crossplane-system
type: Opaque
stringData:
  credentials: |
    {
      "auth": {
        "universal": {
          "client_id": "52eae513-7722-4bf0-af84-54cb1fa7d603",
          "client_secret": "672ae0e2a7b3ee97000d992ffc16726618cb3e4a43776978ca89f33e29ffdda0"
        }
      }
    }
```

#### 2. Create a ProviderConfig

```yaml
apiVersion: infisical.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  host: https://app.infisical.com
  credentials:
    source: Secret
    secretRef:
      namespace: crossplane-system
      name: example-creds
      key: credentials
```

Now you can use `providerConfigRef.name: default` in your managed resources.

You can see the API reference [here](https://doc.crds.dev/github.com/infisical/provider-infisical).

## Developing

Run code-generation pipeline:

```console
go run cmd/generator/main.go "$PWD"
```

Run against a Kubernetes cluster:

```console
make run
```

Build, push, and install:

```console
make all
```

Build binary:

```console
make build
```

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/infisical/provider-infisical/issues).

---

Let me know if you want a fancy badge or `kubectl kustomize` support section next, bruh.
