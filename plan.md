1. **Model USE Flags in `SiteData` and Create `AggUseFlag`**
   - Create `AggUseFlag` in `cmd/g2/site.go` (similar to `AggLicense`).
   - Parse `profiles/use.desc` and `profiles/use.local.desc` to get descriptions, mapped by flag name, and for local desc mapped by `pkg:flag`.
   - Update `SiteData` struct to hold the parsed `UseDesc` and `UseLocalDesc` info, or pass them down into the view model.
2. **Implement REQUIRED_USE Parser**
   - We need a `REQUIRED_USE` parser. Write `required_use.go` with an AST-like structure or just string-based parsing that allows us to get the original form, evaluated form, and string representation.
   - Using the type-switched variadic system mentioned in the prompt (and blog) to design the parser configuration.
3. **Aggregate USE Flags (Global and Repo levels)**
   - In `generateSite()` and `cmd/g2/site_serve.go`, aggregate USE flags from packages, just like `Licenses`.
   - We extract `IUSE` and `REQUIRED_USE` from ebuild vars.
   - For every flag found, add the package to the `AggUseFlag`. If the flag is in `use.desc`, it gets the global description. If there's a local description for the specific package, record it.
   - Add warnings for discrepancies:
     - USE flag in ebuild but not in metadata.xml (Lint rule already does this? We need to display it on the flag page).
     - USE flag not in `use.desc` but in metadata/ebuild (Warning on the flag page).
     - USE flag in `use.desc` but not used anywhere (Warning at the bottom of the use flag profile page).
4. **Generate `/uses/` and `/uses/<flag>/` pages**
   - Create templates `uses.html` and `use.html` (and `repo_uses.html`, `repo_use.html`).
   - List all flags, and for each flag list the packages that use them.
   - Inside the use flag page, list all descriptions used and by who/what.
5. **Update Package UI (IUSE)**
   - On the package list page (`ebuild_details.html` or `package.html`), modify IUSE display. Instead of raw text, show conditional uses as a little note (an icon with hover text showing the condition).
6. **Testing**
   - Write tests for REQUIRED_USE parser.
   - Validate against Gentoo and GURU repos. Run `g2 site serve` locally to test.
