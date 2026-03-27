1. **Create `thirdpartymirrors` parser:**
   - Add a new file `thirdpartymirrors.go` in the root `g2` package.
   - Implement `ParseThirdPartyMirrors(path string) (map[string][]string, error)` and `ParseThirdPartyMirrorsFromReader(r io.Reader) (map[string][]string, error)` to parse the `profiles/thirdpartymirrors` format. The format maps a mirror name to a list of mirror URLs.
   - Add a test for this in `thirdpartymirrors_test.go`.

2. **Integrate into Site Generator (`cmd/g2/site.go`):**
   - Add `ThirdPartyMirrors map[string][]string` to `SiteData`.
   - In `parseRepo`, look for `profiles/thirdpartymirrors`. If it exists, parse it and store it in `SiteData`.

3. **Provide Resolved Mirror URLs to Templates:**
   - The user requested: "The ebuild should have the list of applicable mirrors, while the manifest should have the resolved links."
   - In `buildManifestData` inside `cmd/g2/site.go`, we resolve `mirror://[mirror_name]/[filename]` using `SiteData.ThirdPartyMirrors`.
   - We need to pass `ThirdPartyMirrors` to `buildManifestData`.
   - Actually, wait, the user said: "The ebuild should have the list of applicable mirrors, while the manifest should have the resolved links."
   - Let's add a function to resolve the links. In `ManifestEntryData`, we can add a field `ResolvedURLs []string` alongside `URLs`.
   - Also for the ebuild template `ebuild_details.html`, the user said "The ebuild should have the list of applicable mirrors". This probably means showing which mirrors apply to the `SRC_URI` of the ebuild. We can pass the applicable mirrors to the template, perhaps by computing them in `parseRepo` and adding them to `VersionData` or by a template helper function.
   - Let's compute `ApplicableMirrors map[string][]string` for each `VersionData` (or just in the template data). It will contain the `[mirror_name]` as key and `[]string` of mirror URLs as values.

4. **Update Templates:**
   - Update `ebuild_details.html` to display the "Applicable Mirrors" list if there are any `mirror://` links in `SRC_URI`.
   - Update `repo_package_manifest.html` to show the `ResolvedURLs` (which are the mirror URLs appended with the filename).
   - Update `repo_package.html` manifest table? The user said "Not in the manifest table itself however. The ebuild should have the list of applicable mirrors, while the manifest should have the resolved links."
     - Manifest table in `ebuild_details.html` should show what? `repo_package.html` has a manifest table. The user probably means the `/manifest/` page should have resolved links, not the table in the package or ebuild page.

5. **Pre-commit Checks:**
   - Run tests, gofmt, golangci-lint via the provided `pre_commit_instructions`.
