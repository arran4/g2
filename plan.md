1. **Migration of General Models**: Move `SiteData`, `LicenseData`, `Breadcrumb`, `CategoryData`, `FileData`, `ManifestEntryData`, `PackageData`, `PkgUseFlag`, and `VersionData` from `cmd/g2/site.go` and `cmd/g2/models_page.go` into `site.go` in the root `g2` package. Keep CLI-specific models like `AggCategory`, `AggPackage` in `cmd/g2/`.

2. **Refactor `parseRepo`**:
   - Break `parseRepo` (currently ~400 lines) into smaller files in `cmd/g2/` (e.g., `cmd/g2/parse_repo.go`).
   - Create helper functions:
     - `parseRepoMetadata(sysFS, repoDir, repoName)`
     - `parseRepoProfiles(sysFS, repoDir)`
     - `parseRepoCategoriesAndPackages(sysFS, repoDir, repoName, fastGit, remoteURL, supportedCategories, site)`
   - Add comments explaining their purpose.
   - Use `slices` or `maps` only if it increases readability (e.g., replacing manual loops for sorting or searching).

3. **Refactor `generateSite`**:
   - Move `generateSite` and related functions into `cmd/g2/generate_site.go`.
   - Break `generateSite` to use `errgroup` to run `generateRepoPages` concurrently across repositories.

4. **Upgrade Data Provided to Templates**:
   - For `generatePackagePages` and `generateRepoMovedPackagesPages` (and similar rendering functions), replace manual `Breadcrumb` and `BaseURL` creation with `PageNode`.
   - Update `PageNode` or add a helper to easily attach a generic context like so: `ctx := pageNode.Context("Title", version, genInfo, data)`. The relative URLs will be programmatically determined by `PageNode.BaseURL()` and `PageNode.Breadcrumbs()`.

5. **Testing & Validation**:
   - Build the CLI `go build ./cmd/g2`.
   - Run tests `go test ./cmd/g2/...`.

6. **Pre-commit**: Complete pre commit steps to make sure proper testing, verifications, reviews and reflections are done.
