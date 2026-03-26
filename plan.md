1. Modify `cmd/g2/site.go`:
   - Pass `sortedProfiles` to the `dashboard.html` template rendering as `"Profiles": sortedProfiles,`.
   - Wrap the Profiles generation code inside `if len(sortedProfiles) > 0 { ... }`.
   - Wrap the Repo Profiles generation code inside `if len(site.Profiles) > 0 { ... }`.
2. Modify `cmd/g2/sitegen_templates/repo_index.html`:
   - Conditionally render "Profiles" link only `{{if gt (len .Repo.Profiles) 0}}`.
3. Modify `cmd/g2/sitegen_templates/dashboard.html`:
   - Conditionally render "Profiles" link only `{{if gt (len .Profiles) 0}}`.
4. Pre-commit check.
