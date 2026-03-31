1. **Understand Goal**: I need to add `emerge` command copy-paste hints to `repo_package.html` and `ebuild_details.html`. The prompt specifically mentioned URLs ending in `categories/app-admin/packages/chezmoi-bin/` (the repo package page) and `categories/app-admin/packages/chezmoi-bin/ebuild/2.70.0/` (the ebuild details page). I also need to include instructions for unmasking tools.

2. **Emerge commands**:
   - For a package level (`repo_package.html`): `emerge -a {{.Package.Category}}/{{.Package.Name}}`
   - For a specific version (`ebuild_details.html`): `emerge -a ={{.Package.Category}}/{{.Package.Name}}-{{.VersionData.Version}}`

3. **Unmasking tools**:
   The standard tool for unmasking is `autounmask` (from `app-portage/autounmask`). Gentoo users also use `emerge --autounmask-write ...`. But specifically mentioning "unmasking tool instructions" could mean:
   ```html
   <h3>Install</h3>
   <p>To install this package, run:</p>
   <pre><code>emerge -a {{.Package.Category}}/{{.Package.Name}}</code></pre>
   <p>If the package is masked, you can use the <code>autounmask</code> tool (from <code>app-portage/autounmask</code>) or the <code>--autounmask-write</code> flag:</p>
   <pre><code>emerge --autounmask-write -a {{.Package.Category}}/{{.Package.Name}}</code></pre>
   ```

4. **Let's review the prompt**. It says: "Provide `emerge` command copy paste hints here: ... Including unmasking tool instructions"
   I will add an "Install" section to both HTML templates containing these copy-paste hints.

5. **Editing `templates/views/repo_package.html`**:
   Insert before "Package Information":
   ```html
   <div class="metadata-section">
       <h3>Install</h3>
       <pre><code>emerge -a {{.Package.Category}}/{{.Package.Name}}</code></pre>
       <p>To unmask this package (if needed), use the <code>autounmask</code> tool or standard emerge options:</p>
       <pre><code>emerge --autounmask-write -a {{.Package.Category}}/{{.Package.Name}}</code></pre>
   </div>
   ```

6. **Editing `templates/views/ebuild_details.html`**:
   Insert before "Package Information":
   ```html
   <div class="metadata-section">
       <h3>Install this version</h3>
       <pre><code>emerge -a ={{.Package.Category}}/{{.Package.Name}}-{{.VersionData.Version}}</code></pre>
       <p>To unmask this version (if needed), use the <code>autounmask</code> tool or standard emerge options:</p>
       <pre><code>emerge --autounmask-write -a ={{.Package.Category}}/{{.Package.Name}}-{{.VersionData.Version}}</code></pre>
   </div>
   ```
