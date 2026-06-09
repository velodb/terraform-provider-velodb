---
page_title: "Using the VeloDB provider with Pulumi (incl. Java)"
subcategory: ""
description: |-
  How to consume the VeloDB Terraform provider from Pulumi — in any Pulumi
  language, including Java — using pulumi-terraform-bridge.
---

# Using the VeloDB provider with Pulumi

The VeloDB Terraform provider can be consumed from **Pulumi** (TypeScript, Python,
Go, .NET, **Java**) via
[`pulumi-terraform-bridge`](https://github.com/pulumi/pulumi-terraform-bridge).
The bridge wraps this provider at runtime and code-generates typed SDKs from its
schema — you do not re-implement any resources.

This guide walks through standing up a small "bridge module" and using the
generated **Java** SDK end to end.

## Why this is maintainable

- **Single source of truth** — all resources, inputs, and CRUD behavior stay in
  this Terraform provider. The bridge consumes it as a normal Go-module
  dependency; you upgrade by bumping one version and regenerating.
- **Automatic token mapping** — `tokens.SingleModule` maps every Terraform
  resource/data source into Pulumi mechanically, so new resources added here flow
  into the Pulumi SDKs with no per-resource code.

This provider exposes its entry point through a small, non-`internal` **`shim`**
package precisely so the bridge (a separate Go module) can import it:

```go
// github.com/velodb/terraform-provider-velodb/shim
func NewProvider(version string) func() provider.Provider
```

## Prerequisites

- **Go 1.24+** and the **Pulumi CLI** (the Java language runtime is bundled).
- For Java: a **JDK 17+** to run Gradle 9 (the SDK compiles to Java 11), and
  **Gradle** on `PATH` (`brew install gradle`).

## 1. Create the bridge module

Create a sibling Go module (e.g. `pulumi-velodb`) with this layout:

```
pulumi-velodb/
├── Makefile
└── provider/
    ├── go.mod
    ├── resources.go
    ├── pkg/version/version.go
    └── cmd/
        ├── pulumi-tfgen-velodb/main.go
        └── pulumi-resource-velodb/main.go   (+ bridge-metadata.json, schema.json)
```

`provider/go.mod`:

```go
module github.com/velodb/pulumi-velodb/provider

go 1.24.0

require (
    github.com/pulumi/pulumi-terraform-bridge/v3 v3.132.0
    github.com/velodb/terraform-provider-velodb v1.1.4
)

// Pulumi's fork of terraform-plugin-sdk is required transitively.
replace github.com/hashicorp/terraform-plugin-sdk/v2 => github.com/pulumi/terraform-plugin-sdk/v2 v2.0.0-20260318212141-5525259d096b
```

`provider/resources.go` — this is the only file with real logic:

```go
package velodb

import (
    _ "embed"

    pfbridge "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
    "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge"
    "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/tfbridge/tokens"

    velodbshim "github.com/velodb/terraform-provider-velodb/shim"

    "github.com/velodb/pulumi-velodb/provider/pkg/version"
)

//go:embed cmd/pulumi-resource-velodb/bridge-metadata.json
var metadata []byte

func Provider() tfbridge.ProviderInfo {
    prov := tfbridge.ProviderInfo{
        // Plugin-framework entry point via the shim package.
        P:            pfbridge.ShimProvider(velodbshim.NewProvider(version.Version)()),
        Name:         "velodb",
        Version:      version.Version,
        DisplayName:  "VeloDB",
        Publisher:    "VeloDB",
        Description:  "A Pulumi package for creating and managing VeloDB Cloud resources.",
        License:      "Apache-2.0",
        Homepage:     "https://www.velodb.io",
        Repository:   "https://github.com/velodb/pulumi-velodb",
        GitHubOrg:    "velodb",
        MetadataInfo: tfbridge.NewProviderMetadata(metadata),
        // api_key is already marked Sensitive in the Terraform schema, so the
        // bridge carries it through as a secret automatically — no override needed.
        Java: &tfbridge.JavaInfo{BasePackage: "com.velodb"},
    }

    // Map every velodb_* resource/data source into the index module:
    // velodb_warehouse -> velodb:index:Warehouse, etc. New resources map for free.
    prov.MustComputeTokens(tokens.SingleModule("velodb_", "index", tokens.MakeStandard("velodb")))
    prov.MustApplyAutoAliases()
    prov.SetAutonaming(255, "-")
    return prov
}
```

`provider/cmd/pulumi-tfgen-velodb/main.go`:

```go
package main

import (
    "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfgen"
    velodb "github.com/velodb/pulumi-velodb/provider"
)

func main() { tfgen.Main("velodb", velodb.Provider()) }
```

`provider/cmd/pulumi-resource-velodb/main.go`:

```go
package main

import (
    "context"
    _ "embed"

    "github.com/pulumi/pulumi-terraform-bridge/v3/pkg/pf/tfbridge"
    velodb "github.com/velodb/pulumi-velodb/provider"
)

//go:embed schema.json
var pulumiSchema []byte

func main() {
    tfbridge.Main(context.Background(), "velodb", velodb.Provider(),
        tfbridge.ProviderMetadata{PackageSchema: pulumiSchema})
}
```

Seed `bridge-metadata.json` with `{"auto-aliasing":{"resources":{},"datasources":{}},"auto-settings":{}}`
and `schema.json` with `{}` so the `//go:embed` directives compile before the
first generation.

## 2. Generate the schema and the Java SDK

```bash
# Build the codegen binary and write schema.json + bridge-metadata.json
cd provider && go build -o ../bin/pulumi-tfgen-velodb ./cmd/pulumi-tfgen-velodb && cd ..
./bin/pulumi-tfgen-velodb schema --out provider/cmd/pulumi-resource-velodb

# Generate the Java SDK
./bin/pulumi-tfgen-velodb java --out sdk/java

# Build the provider plugin binary (embeds schema.json)
cd provider && go build -o ../bin/pulumi-resource-velodb ./cmd/pulumi-resource-velodb && cd ..
```

The generated schema maps every resource and data source:

| Terraform | Pulumi token | Java |
|---|---|---|
| `velodb_warehouse` | `velodb:index:Warehouse` | `Warehouse` |
| `velodb_cluster` | `velodb:index:Cluster` | `Cluster` |
| `velodb_warehouse_public_access_policy` | `velodb:index:WarehousePublicAccessPolicy` | `WarehousePublicAccessPolicy` |
| `velodb_private_link_endpoint_service` | `velodb:index:PrivateLinkEndpointService` | `PrivateLinkEndpointService` |
| `velodb_warehouse_private_endpoint` | `velodb:index:WarehousePrivateEndpoint` | `WarehousePrivateEndpoint` |
| `velodb_warehouses` (data source) | — | `VelodbFunctions.getWarehouses()` |
| `velodb_clusters`, `velodb_warehouse_connections`, `velodb_warehouse_versions`, `velodb_private_link_endpoint_services` | — | `getClusters`, `getWarehouseConnections`, `getWarehouseVersions`, `getPrivateLinkEndpointServices` |

## 3. Use the Java SDK

Publish the SDK to your local Maven repo and put the plugin on `PATH`:

```bash
cd sdk/java && \
  JAVA_HOME=$(/usr/libexec/java_home -v 17) \
  PACKAGE_VERSION=1.0.0 \
  gradle publishToMavenLocal \
    -Dorg.gradle.java.installations.paths=$(/usr/libexec/java_home -v 11)
cd ../..
export PATH="$PWD/bin:$PATH"   # ambient pulumi-resource-velodb
```

A Pulumi Java project:

`Pulumi.yaml`

```yaml
name: my-velodb-app
runtime: java
```

`build.gradle`

```groovy
plugins { id 'java'; id 'application' }
repositories { mavenLocal(); mavenCentral() }   // drop mavenLocal once published to Maven Central
dependencies {
    implementation 'com.velodb:velodb:1.0.0'
    implementation 'com.pulumi:pulumi:1.0.0'
}
application { mainClass = 'com.example.App' }
```

`src/main/java/com/example/App.java` — read-only example (creates nothing):

```java
package com.example;

import com.pulumi.Pulumi;
import com.velodb.velodb.VelodbFunctions;
import java.util.stream.Collectors;

public class App {
    public static void main(String[] args) {
        Pulumi.run(ctx -> {
            var warehouses = VelodbFunctions.getWarehouses();
            ctx.export("warehouseTotal", warehouses.applyValue(r -> r.total()));
            ctx.export("warehouseNames", warehouses.applyValue(r ->
                r.warehouses().stream().map(w -> w.name()).collect(Collectors.toList())));
        });
    }
}
```

Configure credentials and run against a local file backend (no Pulumi Cloud
account required):

```bash
export VELODB_API_KEY=vdb_sk_xxx           # or: pulumi config set --secret velodb:apiKey ...
export VELODB_HOST=api.velodb.cloud        # default; override for other environments
export PULUMI_BACKEND_URL="file://$PWD/.pulumi-state"
export PULUMI_CONFIG_PASSPHRASE="<choose-one>"
export JAVA_HOME=$(/usr/libexec/java_home -v 17)

pulumi stack init dev
pulumi up --yes
# Outputs: warehouseTotal, warehouseNames
```

### Creating a resource

```java
import com.velodb.velodb.Warehouse;
import com.velodb.velodb.WarehouseArgs;

var wh = new Warehouse("example", WarehouseArgs.builder()
    .name("pulumi-demo")
    .deploymentMode("SAAS")     // required: deploymentMode, cloudProvider, region
    .cloudProvider("aws")
    .region("us-east-1")
    .build());
```

## Publishing (optional)

- **Java SDK → Maven Central**: the generated `sdk/java/build.gradle` is pre-wired
  for the Sonatype Central Portal (coordinates `com.velodb:velodb`, GPG signing,
  sources+javadoc jars). Supply `SIGNING_KEY`/`SIGNING_PASSWORD`,
  `PUBLISH_REPO_USERNAME`/`PUBLISH_REPO_PASSWORD`, `PACKAGE_VERSION`, fill the POM
  identity fields, then `gradle publishToSonatype`.
- **Plugin binary**: cross-compile `pulumi-resource-velodb` per OS/arch and attach
  to a GitHub Release so `pulumi` downloads the runtime automatically.

## Notes & gotchas

- On Linux, replace `/usr/libexec/java_home -v <ver>` with direct JDK paths.
- The Pulumi Java codegen targets a Java 11 toolchain, but Gradle 9 requires a
  JDK 17+ to *run* — provide both (run under 17, point at an 11 toolchain via
  `-Dorg.gradle.java.installations.paths`).
- If the generated `sdk/java/settings.gradle` contains a stray `include("lib")`
  while `build.gradle`/`src` are at the SDK root, remove that line (handle it with
  a post-generation hook so it survives regeneration).
