1. **Extend `NewsItem` struct in `cmd/g2/site.go`**:
   - Add fields like `NewsItemFormat`, `Translator`, `DisplayIfInstalled`, `DisplayIfKeyword`, `DisplayIfProfile`, `BodyHTML`.

2. **Update news parsing in `cmd/g2/site.go`**:
   - Extract those new headers during parsing.
   - If `News-Item-Format: 2.0` is present, process `item.Body` to create `item.BodyHTML` using a simple markdown-like parser (handles unordered lists and code blocks). Otherwise, fallback to plain text wrapped in template.HTML (or handle uniformly in templates).

3. **Update Templates**:
   - `cmd/g2/sitegen_templates/news_dashboard.html`: Use `{{.BodyHTML}}` instead of `{{.Body}}` and perhaps omit the `style="white-space: pre-wrap;"` if it's HTML. (Or use it intelligently).
   - `cmd/g2/sitegen_templates/news_article.html`: Show the new fields (Translators, DisplayIf*, Format) if they exist. Use `{{.NewsItem.BodyHTML}}` instead of `{{.NewsItem.Body}}`.

4. **Testing**:
   - Add a test or verify with local news items (we can create a test case).
   - Ensure the `pre_commit_instructions` tool is called and followed.
