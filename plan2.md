1. Pass `len(sortedLicenses) > 0` and `totalPackages > 0` and `len(sortedProjects) > 0` etc. to `repo_index.html`?
Actually, `site.go` has `totalPackages`, `sortedLicenses`, `sortedProjects`. Let's pass:
- `"GlobalLicensesCount": len(sortedLicenses)`
- `"GlobalPackagesCount": totalPackages`
- `"GlobalProfilesCount": len(sortedProfiles)`
- `"GlobalCategoriesCount": len(sortedCategories)`
- `"GlobalProjectsCount": len(sortedProjects)`

Then in `repo_index.html`, for Categories:
```html
    {{if gt (len .Repo.Categories) 0}}
    <li><a href="categories/">Categories ({{len .Repo.Categories}})</a></li>
    {{else if gt .GlobalCategoriesCount 0}}
    <li><a href="../../categories/">Categories (Global)</a></li>
    {{end}}
```
For Packages:
```html
    {{if gt .PackageCount 0}}
    <li><a href="packages/">Packages ({{.PackageCount}})</a></li>
    {{else if gt .GlobalPackagesCount 0}}
    <li><a href="../../packages/">Packages (Global)</a></li>
    {{end}}
```
For Licenses:
```html
    {{if gt .GlobalLicensesCount 0}}
    <li><a href="../../licenses/">Licenses (Global)</a></li>
    {{end}}
```
(Wait, we don't have local licenses, right? So it's only Global).

For Profiles:
```html
    {{if gt (len .Repo.Profiles) 0}}
    <li><a href="profiles/">Profiles</a></li>
    {{else if gt .GlobalProfilesCount 0}}
    <li><a href="../../profiles/">Profiles (Global)</a></li>
    {{end}}
```

Wait, does the global page have projects?
We should also link projects? (Currently no projects link in `repo_index.html`). The user asked "Can we apply this to everything that it makes sense to?".

Let's check `dashboard.html` as well.
Categories, Packages, Licenses, Projects, Profiles.
If there are 0 projects, don't show the link in `dashboard.html`.

Wait, for Categories, Packages, Licenses, Projects on `dashboard.html`, we already have `len .Categories`, `len .Packages`, etc.
So in `dashboard.html`:
```html
        {{if gt (len .Categories) 0}}
        <li><a href="categories/">Categories ({{len .Categories}})</a></li>
        {{end}}
        {{if gt (len .Packages) 0}}
        <li><a href="packages/">Packages ({{len .Packages}})</a></li>
        {{end}}
        {{if gt (len .Licenses) 0}}
        <li><a href="licenses/">Licenses ({{len .Licenses}})</a></li>
        {{end}}
        {{if gt (len .Projects) 0}}
        <li><a href="projects/">Projects ({{len .Projects}})</a></li>
        {{end}}
        {{if gt (len .Profiles) 0}}
        <li><a href="profiles/">Profiles</a></li>
        {{end}}
```
(Overlays also? `len .Repos`).

Let's implement this.
