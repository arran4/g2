1. **Understand the request**
   - The user wants the repository stats to be moved to a separate page, either `/repo/stats/index.html` or `/repo/stats.html`.
   - The user also wants a link to this new stats page from the repository index page (`/repo/index.html`).

2. **Modify `cmd/g2/site.go` to generate the new stats page per repo**
   - In `generateSite`, inside the `for _, site := range sites` loop (repo-specific generation), add logic to generate a `stats/index.html` page for each repository.
   - The output path will be `filepath.Join(repoDir, "stats", "index.html")`.
   - Pass the necessary `SiteData` (and possibly `GenInfo`) to the template.

3. **Create the new `templates/site/repo_stats.html` template**
   - Create a new template file for the repository stats page.
   - Move the "Repository Details" table (containing Git Size, Checkout Time, Processing Time, Source URL) from `repo_index.html` to this new template.

4. **Update `templates/site/repo_index.html`**
   - Remove the stats table from `repo_index.html`.
   - Add a link to the new stats page, e.g., `<a href="stats/">Repository Statistics</a>`.

5. **Update `cmd/g2/site_serve.go` (if necessary)**
   - Check if `site_serve.go` needs a new route for `/repos/{repo}/stats/` to serve the new stats page dynamically.

6. **Testing and Verification**
   - Run `go build ./...` and `go test ./...`.
   - Generate the site locally and verify the new page and link exist.
   - Take a screenshot of the modified `repo_index.html` and the new `repo_stats.html` using the Playwright script.

7. **Reply to PR comment and submit**
   - Reply to the PR comment indicating the changes have been made.
   - Commit and submit the changes to the same branch.
