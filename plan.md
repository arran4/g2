1. Add `templates/ebuild/generate.tmpl`. Wait, maybe name it `generate.tmpl` but `templates/embed.go` only embeds `*.ebuild`. Oh, wait! I can just embed `ebuild/*.tmpl` or `ebuild/*` if I want to embed `generate.tmpl`. Alternatively, since the other files are `*.ebuild` skeletons, maybe `generate.tmpl` is fine and I just need to modify `templates/embed.go`.
Let's see: `//go:embed ebuild/*.ebuild ebuild/*.tmpl`.
Or maybe just rename the template `string_output.tmpl` or `generate.tmpl`.
Let's call it `generate.tmpl`.
2. Update `templates/embed.go` to include `//go:embed ebuild/*.tmpl`.
3. Update `ebuild.go` to parse `templates.EbuildFS` and the `generate.tmpl` file.
Let's do this to resolve the PR comment.
