---
page_title: "How to publish the next VeloDB provider version"
subcategory: ""
description: |-
  Maintainer steps for publishing the next VeloDB Terraform provider version
  using the existing GitHub Actions and Terraform Registry setup.
---

# How to publish the next VeloDB provider version

This guide describes the routine release process for
`velodb/terraform-provider-velodb` after the one-time setup is already in place.

The repository already has:

- `terraform-registry-manifest.json`
- `.goreleaser.yml`
- `.github/workflows/release.yml`
- GitHub Actions secrets `GPG_PRIVATE_KEY` and `PASSPHRASE`
- A matching GPG public key registered in Terraform Registry for the `velodb`
  namespace
- The provider published at `registry.terraform.io/velodb/velodb`

If the GitHub Actions secrets are missing, expired, or need to be rotated,
contact the repository administrator: GitHub ID `morningman`.

Use this guide when publishing the next provider version.

## 1. Choose the next version

Check the latest tag:

```bash
git fetch --tags origin
git tag --sort=-version:refname | head -n 10
```

Choose the next semantic version. For example, if the latest published version
is `v1.1.5`, the next patch version is:

```text
v1.1.6
```

Do not reuse an existing version tag. Terraform Registry will not ingest the
same version as a new release.

## 2. Prepare the release commit

Start from `main` and make sure it is up to date:

```bash
git checkout main
git pull --ff-only origin main
```

Make and merge any code or documentation changes that should be included in the
release. The release workflow builds from the commit pointed to by the tag, so
tag the exact commit you want to publish.

Before tagging, run the normal project checks:

```bash
go mod tidy
go test ./...
```

If `go mod tidy` changes `go.mod` or `go.sum`, review and commit those changes
before creating the tag.

## 3. Create and push the version tag

Replace `v1.1.6` with the version you chose:

```bash
git tag v1.1.6
git push origin v1.1.6
```

Pushing the tag triggers the `Release` GitHub Actions workflow. The workflow
runs GoReleaser, signs the checksum file with the configured GPG key, and
creates a GitHub Release.

## 4. Verify the GitHub Actions workflow

Open the repository in GitHub, then go to:

```text
Actions -> Release
```

Open the workflow run for the tag you pushed and confirm it completed
successfully.

If the workflow fails, do not create another tag immediately. Fix the failure
on `main`, then publish a new version tag. Do not delete and recreate a tag that
Terraform Registry may already have seen.

## 5. Verify the GitHub Release assets

Open the GitHub Release for the new version. Confirm the release contains these
asset types:

```text
terraform-provider-velodb_<version>_<os>_<arch>.zip
terraform-provider-velodb_<version>_SHA256SUMS
terraform-provider-velodb_<version>_SHA256SUMS.sig
terraform-provider-velodb_<version>_manifest.json
```

For `v1.1.6`, examples include:

```text
terraform-provider-velodb_1.1.6_linux_amd64.zip
terraform-provider-velodb_1.1.6_SHA256SUMS
terraform-provider-velodb_1.1.6_SHA256SUMS.sig
terraform-provider-velodb_1.1.6_manifest.json
```

Terraform Registry requires the zip archives, checksum file, detached GPG
signature, and manifest file.

## 6. Verify Terraform Registry ingestion

Terraform Registry receives new versions from the GitHub release webhook. Wait a
few minutes, then open:

```text
https://registry.terraform.io/providers/velodb/velodb/latest
```

Confirm that the latest version is the version you just published.

If the version does not appear after several minutes, open the provider settings
in Terraform Registry and use the resync option. Also confirm that the GitHub
Release has all required assets.

## 7. Test installation from Terraform Registry

Create a clean temporary Terraform project:

```bash
mkdir -p /tmp/velodb-provider-install-test
cd /tmp/velodb-provider-install-test
```

Create `main.tf`:

```hcl
terraform {
  required_providers {
    velodb = {
      source  = "velodb/velodb"
      version = "~> 1.1"
    }
  }
}

provider "velodb" {
  # api_key can also be set with VELODB_API_KEY.
}
```

Run:

```bash
terraform init
```

The command should download the provider from:

```text
registry.terraform.io/velodb/velodb
```

For an exact version test, pin the new version:

```hcl
version = "= 1.1.6"
```

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| The `Release` workflow did not start. | The tag was not pushed, the tag does not match `v*`, or GitHub Actions is disabled. | Push a tag such as `v1.1.6` and confirm Actions are enabled. |
| The workflow cannot import the GPG key. | `GPG_PRIVATE_KEY` is incomplete, or `PASSPHRASE` is wrong. | Reconfigure the repository secrets with the matching private key and passphrase. |
| The GitHub Release is missing assets. | GoReleaser failed or the workflow stopped before release upload. | Fix the workflow failure on `main`, then publish a new version tag. |
| Terraform Registry rejects the release signature. | The GPG public key in Terraform Registry does not match the private key used by GitHub Actions. | Add the matching public key to the `velodb` namespace in Terraform Registry. |
| Terraform Registry does not show the new version. | The GitHub release webhook did not ingest the version, or required assets are missing. | Confirm the release assets, then use the Registry provider settings to resync. |
| `terraform init` reports a protocol mismatch. | The Registry manifest has the wrong protocol version. | Keep `metadata.protocol_versions` set to `["6.0"]`. |
