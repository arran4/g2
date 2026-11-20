# GitHub Actions Pages Deployment Examples

> **Note:** These files are starter examples only, not production guarantees or active `g2` CI workflows. Copy the workflow you need into your own repository and pin `arran4/g2-action` or the `g2` release version if strict reproducibility is required.

## Prerequisites
1. In your GitHub repository, open **Settings → Pages → Build and deployment**.
2. Set **Source** to **GitHub Actions** (these workflows upload and deploy Pages artifacts directly rather than pushing to a `gh-pages` branch).

## Available Examples

### 1. Single-Overlay Site (`pages-single-overlay.yml`)
- Copy `examples/github-actions/pages-single-overlay.yml` to `.github/workflows/deploy-pages.yml` in your overlay repository.
- Runs `g2 overlay site generate -out _site .` and validates `_site/index.html` and `_site/stats/index.html`.
- Uses least-privilege permissions (`contents: read` on the `build` job; `pages: write` and `id-token: write` on the `deploy` job).

### 2. Multi-Overlay Aggregated Site (`pages-multi-overlay.yml`)
- Copy `examples/github-actions/pages-multi-overlay.yml` to `.github/workflows/deploy-pages.yml` in your site repository.
- Reads `repositories.xml` from the `REPOSITORIES_SOURCE` repository variable (or manual `workflow_dispatch` input `repositories_source`, defaulting to `repositories.xml`).
- Runs `g2 overlays site generate -out _site -concurrency 4 -work-mode temp "$REPOSITORIES_SOURCE"`.
- Includes an optional commented `schedule` trigger for periodic upstream refreshes.

## Customization & Subpath Deployments
- **Pinning `g2` version:** Change `version: latest` under `arran4/g2-action@v1` to a specific tag (e.g. `v0.0.107`).
- **Project vs. Root GitHub Pages:** Generated links are relative (`BaseURL`), so the `_site` output works on both root user/org sites (`https://<user>.github.io/`) and project subpath sites (`https://<user>.github.io/<repo>/`) without changing flags.
- **Strict link checking (if enabled in your release):** You can optionally append `-strict-links` to the generate command in CI once your overlay metadata has no missing references.
