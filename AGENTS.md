# AGENTS.md

Rules for working in this repo (VeloDB Terraform provider + BYOC modules).

## Commands

- `make build` · `make test` · `make lint` · `make fmt`
- Run `terraform fmt` after editing anything under `modules/` or `examples/`.

## Rules

1. **Names are consistent across the whole project.** The same thing uses the same name in the provider resources, the modules, the examples, and the docs. Prefer specific, unambiguous names over generic ones.
2. **Every feature is supported in both the resources and the modules.** When you add or change a capability on a resource, expose it in the modules too — under the same name.
3. **Keep docs and examples in sync with the code.** Update them in the same change.
4. Before finishing: `make build`, `make test`, `make lint`, and `terraform fmt`.
