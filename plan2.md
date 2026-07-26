1. Issue 1 (Comment ID 3651402337): We need to parse all ebuilds in the package directory, not just those in `pkg.Versions`. `pkg.Versions` might be filtered by the `lint query` command, which causes valid distfiles for other versions to be marked as orphaned. We should iterate through the actual directory (`filepath.Join(repoDir, pkg.Category, pkg.Name)`) and parse all `.ebuild` files to populate `usedFiles`.
2. Issue 2 (Comment ID 3651402440): `ExtractURIs` is incomplete as it misses `+="...` assignments. The rule should rely on a fully parsed `SRC_URI`. `g2.ParseEbuild(..., ParseFull)` handles SRC_URI accumulation natively (it populates `Ebuild.SrcUri`). We can use `g2.ParseEbuild` on each `.ebuild` file we find in the directory to get `Ebuild.SrcUri`. This is much more reliable than `ExtractURIs`.

So the fix plan:
1. In `OrphanedManifestLintRule.LintWithQA`:
   - Replace the loop over `pkg.Versions` with a loop over `os.ReadDir(pkgDir)`.
   - For each file that ends in `.ebuild`:
     - Parse it using `g2.ParseEbuild(os.DirFS(pkgDir), filename, g2.ParseFull)`.
     - Iterate over `parsedEbuild.SrcUri` and add `uri.Filename` to `usedFiles`.
2. Ensure we still check missing files (`AUX`, `EBUILD`, etc.) correctly.
3. Apply changes and verify with `go test` and `go build`.
