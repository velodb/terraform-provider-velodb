# Hosting the docs on Cloudflare Pages

The provider docs live in `docs/` as Markdown. `mkdocs.yml` configures MkDocs (Material theme) to render them into a static site.

## One-time setup

1. Log into the [Cloudflare dashboard](https://dash.cloudflare.com/) → **Workers & Pages** → **Create** → **Pages** → **Connect to Git**
2. Select the `tomz-alt/terraform-provider-velodb` repository
3. Configure the build:

   | Setting | Value |
   |---|---|
   | Production branch | `master` |
   | Build command | `pip install -r requirements.txt && mkdocs build` |
   | Build output directory | `site` |
   | Root directory | `/` |

4. Environment variables (optional):

   | Variable | Value |
   |---|---|
   | `PYTHON_VERSION` | `3.11` |

5. Click **Save and Deploy**

Cloudflare auto-deploys on every push to `master`. Preview URLs also spin up for pull requests.

## Local preview

```bash
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
mkdocs serve
# open http://127.0.0.1:8000
```

## Files involved

- `mkdocs.yml` — site config, navigation, theme
- `requirements.txt` — Python deps Cloudflare installs at build time
- `.python-version` — pins Python version for Cloudflare builds
- `docs/**/*.md` — Markdown source
- `site/` — generated HTML (git-ignored)

## Updating the site

Just edit the Markdown files in `docs/` and push. Cloudflare rebuilds within ~1 min.
