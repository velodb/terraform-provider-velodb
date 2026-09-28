# AGENTS.md

Rules for working in this repo (VeloDB Terraform provider + BYOC modules).

## Commands

- `make build` · `make test` · `make lint` · `make fmt`
- Run `terraform fmt` after editing anything under `modules/` or `examples/`.

## Rules

1. **Names are consistent across the whole project.** The same thing uses the same name in the provider resources, the modules, the examples, and the docs. Prefer specific, unambiguous names over generic ones.
2. **Every feature is supported in both the resources and the modules.** When you add or change a capability on a resource, expose it in the modules too — under the same name.
3. **Keep docs and examples in sync with the code.** Update them in the same change.
4. **Every cloud resource supports both new and existing.** For any cloud resource (VPC, subnets, IAM roles, S3 buckets, etc.), let users either have the module/provider create a new one or bring an existing one — expose both paths in the resources, modules, examples, and docs under consistent names.
5. Before finishing: `make build`, `make test`, `make lint`, and `terraform fmt`.
6. **Terraform attributes round-trip through the API.** If an attribute is accepted during create or update and returned by GET, decode it and populate it during `Read`. Preserve prior state only when the API genuinely cannot return the value, and document that exception.
7. **Test the complete resource lifecycle.** For every managed attribute, cover create → read/refresh and import → read. Verify returned values, null or omitted values, and externally changed values where applicable.
8. **Do not assume replacement is safe.** Before using `RequiresReplace`, verify that the backend supports automatic destroy and recreate. Otherwise, reject the change with a clear error and require an explicit migration.
