package main

const layoutTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body { font-family: sans-serif; margin: 2em; line-height: 1.6; color: #333; }
        h1, h2, h3 { color: #5c4f85; } /* Tyrian purple-ish */
        a { text-decoration: none; color: #0055aa; }
        a:hover { text-decoration: underline; }
        table { border-collapse: collapse; width: 100%; margin-bottom: 2em; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        .nav { margin-bottom: 2em; padding-bottom: 1em; border-bottom: 1px solid #ccc; }
        .container { max-width: 1200px; margin: 0 auto; }
        pre { background-color: #f4f4f4; padding: 1em; overflow-x: auto; border-radius: 4px; }
        .metadata-section { background: #f9f9f9; padding: 1em; border-radius: 5px; margin-bottom: 2em; border: 1px solid #eee; }
    </style>
</head>
<body>
    <div class="container">
        <div class="nav">
            <h1><a href="/">Packages</a></h1>
        </div>
        {{.Content}}
    </div>
</body>
</html>`

const indexTemplate = `
<h2>Categories</h2>
<ul>
    {{range .Categories}}
    <li><a href="{{.Name}}/">{{.Name}}</a> ({{len .Packages}} packages)</li>
    {{end}}
</ul>
`

const categoryTemplate = `
<h2>Category: {{.Category.Name}}</h2>
<ul>
    {{range .Category.Packages}}
    <li><a href="{{.Name}}/">{{.Name}}</a></li>
    {{end}}
</ul>
`

const packageTemplate = `
<h2>{{.Package.Category}}/{{.Package.Name}}</h2>

<div class="metadata-section">
    <h3>Versions</h3>
    <table>
        <tr>
            <th>Version</th>
            <th>EAPI</th>
            <th>Keywords</th>
            <th>Description</th>
        </tr>
        {{range .Package.Versions}}
        <tr>
            <td>{{if .Ebuild.Vars.PV}}{{.Ebuild.Vars.PV}}{{else}}{{.Version}}{{end}}</td>
            <td>{{.Ebuild.Vars.EAPI}}</td>
            <td>{{.Ebuild.Vars.KEYWORDS}}</td>
            <td>{{.Ebuild.Vars.DESCRIPTION}}</td>
        </tr>
        {{end}}
    </table>
</div>

{{if .Package.Metadata}}
<div class="metadata-section">
    <h3>Metadata</h3>
    <pre>{{.Package.Metadata}}</pre>
</div>
{{end}}

{{if .Package.Manifest}}
<div class="metadata-section">
    <h3>Manifest</h3>
    <table>
        <tr>
            <th>Type</th>
            <th>File</th>
            <th>Size</th>
        </tr>
        {{range .Package.Manifest.Entries}}
        <tr>
            <td>{{.Type}}</td>
            <td>{{.Filename}}</td>
            <td>{{.Size}} bytes</td>
        </tr>
        {{end}}
    </table>
</div>
{{end}}
`