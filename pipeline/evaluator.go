package pipeline

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
	"golang.org/x/net/html"
)

// PipelineValue models the structured internal value in the pipeline.
// It explicitly distinguishes zero cardinality, scalars, and lists.
type PipelineValue struct {
	IsEmpty bool
	Value   interface{}
	List    []interface{}
	RawNode *xmlquery.Node
}

// IsList returns true if the value is a list.
func (v *PipelineValue) IsList() bool {
	return !v.IsEmpty && v.List != nil
}

// GetString converts the single scalar value to a string if it exists.
func (v *PipelineValue) GetString() string {
	if v.IsEmpty || v.Value == nil {
		return ""
	}
	switch val := v.Value.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// Evaluator is responsible for executing pipelines.
type Evaluator struct {
	Client *http.Client
}

// NewEvaluator creates a new pipeline evaluator.
func NewEvaluator(client *http.Client) *Evaluator {
	if client == nil {
		client = http.DefaultClient
	}
	return &Evaluator{Client: client}
}

// Evaluate runs the given pipeline string and returns the resulting value.
func (e *Evaluator) Evaluate(pipelineStr string) (*PipelineValue, error) {
	commands, err := TokenizePipeline(pipelineStr)
	if err != nil {
		return nil, err
	}

	val := &PipelineValue{IsEmpty: true}
	var currentURL string

	for _, cmdStr := range commands {
		val, err = e.evaluateCommand(cmdStr, val, &currentURL)
		if err != nil {
			return nil, err
		}
	}

	return val, nil
}

func (e *Evaluator) evaluateCommand(cmdStr string, val *PipelineValue, currentURL *string) (*PipelineValue, error) {
	if strings.HasPrefix(cmdStr, "get(") && strings.HasSuffix(cmdStr, ")") {
		targetURL := cmdStr[4 : len(cmdStr)-1]
		targetURL = strings.Trim(targetURL, "'\"")
		*currentURL = targetURL

		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			return nil, fmt.Errorf("error creating request for %s: %v", targetURL, err)
		}
		req.Header.Set("User-Agent", "g2/pipeline")

		resp, err := e.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error fetching URL %s: %v", targetURL, err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("error fetching URL %s: status %d", targetURL, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response from %s: %v", targetURL, err)
		}

		return &PipelineValue{Value: string(body)}, nil
	}

	switch cmdStr {
	case "rss", "atom":
		if val.IsEmpty || val.IsList() {
			return nil, fmt.Errorf("error parsing %s: need string input", cmdStr)
		}
		doc, err := xmlquery.Parse(strings.NewReader(val.GetString()))
		if err != nil {
			return nil, fmt.Errorf("error parsing %s: %w", cmdStr, err)
		}

		var items []*xmlquery.Node
		if cmdStr == "rss" {
			items = xmlquery.Find(doc, "//item")
		} else {
			items = xmlquery.Find(doc, "//*[local-name()='entry']")
		}

		var list []interface{}
		for _, item := range items {
			list = append(list, item.OutputXML(true))
		}
		return &PipelineValue{List: list}, nil

	case "xml":
		if val.IsEmpty || val.IsList() {
			return nil, fmt.Errorf("error parsing xml: need string input")
		}
		doc, err := xmlquery.Parse(strings.NewReader(val.GetString()))
		if err != nil {
			return nil, fmt.Errorf("error parsing XML: %w", err)
		}
		return &PipelineValue{RawNode: doc, Value: val.Value}, nil

	case "first":
		if val.IsEmpty {
			return val, nil
		}
		if val.IsList() {
			if len(val.List) > 0 {
				return &PipelineValue{Value: val.List[0]}, nil
			}
			return &PipelineValue{IsEmpty: true}, nil
		}
		return val, nil

	case "last":
		if val.IsEmpty {
			return val, nil
		}
		if val.IsList() {
			if len(val.List) > 0 {
				return &PipelineValue{Value: val.List[len(val.List)-1]}, nil
			}
			return &PipelineValue{IsEmpty: true}, nil
		}
		return val, nil

	case "single", "exactly_one":
		if val.IsEmpty || (val.IsList() && len(val.List) == 0) || (!val.IsList() && val.GetString() == "") {
			return nil, fmt.Errorf("exactly_one/single expected 1 item, got 0")
		}
		if val.IsList() {
			if len(val.List) > 1 {
				return nil, fmt.Errorf("exactly_one/single expected 1 item, got %d", len(val.List))
			}
			return &PipelineValue{Value: val.List[0]}, nil
		}
		return val, nil

	case "link":
		if val.IsEmpty || val.IsList() {
			return nil, fmt.Errorf("link requires a string input (XML item)")
		}
		doc, err := xmlquery.Parse(strings.NewReader(val.GetString()))
		if err == nil {
			link := xmlquery.FindOne(doc, "//link")
			if link != nil {
				if href := link.SelectAttr("href"); href != "" {
					return &PipelineValue{Value: href}, nil
				}
				if link.InnerText() != "" {
					return &PipelineValue{Value: strings.TrimSpace(link.InnerText())}, nil
				}
			}
			link = xmlquery.FindOne(doc, "//*[local-name()='link']")
			if link != nil {
				if href := link.SelectAttr("href"); href != "" {
					return &PipelineValue{Value: href}, nil
				}
			}
		}
		return &PipelineValue{IsEmpty: true}, nil

	case "url.basename":
		if val.IsEmpty {
			return val, nil
		}
		if val.IsList() {
			var results []interface{}
			for _, item := range val.List {
				u, err := url.Parse(fmt.Sprintf("%v", item))
				if err == nil && u.Path != "" {
					results = append(results, path.Base(u.Path))
				} else {
					results = append(results, "")
				}
			}
			return &PipelineValue{List: results, IsEmpty: false}, nil
		}
		u, err := url.Parse(val.GetString())
		if err == nil && u.Path != "" {
			return &PipelineValue{Value: path.Base(u.Path), IsEmpty: false}, nil
		}
		return &PipelineValue{Value: "", IsEmpty: false}, nil

	case "trim":
		if val.IsEmpty {
			return val, nil
		}
		if val.IsList() {
			var results []interface{}
			for _, item := range val.List {
				results = append(results, strings.TrimSpace(fmt.Sprintf("%v", item)))
			}
			return &PipelineValue{List: results, IsEmpty: false}, nil
		}
		return &PipelineValue{Value: strings.TrimSpace(val.GetString()), IsEmpty: false}, nil

	case "html_links":
		if val.IsEmpty || val.IsList() {
			return val, nil
		}
		doc, err := html.Parse(strings.NewReader(val.GetString()))
		if err != nil {
			return val, nil
		}
		var links []interface{}
		var f func(*html.Node)
		f = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "a" {
				for _, a := range n.Attr {
					if a.Key == "href" {
						link := a.Val
						if *currentURL != "" {
							base, err := url.Parse(*currentURL)
							if err == nil {
								u, err := url.Parse(link)
								if err == nil {
									link = base.ResolveReference(u).String()
								}
							}
						}
						links = append(links, link)
						break
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				f(c)
			}
		}
		f(doc)
		return &PipelineValue{List: links}, nil
	}

	if strings.HasPrefix(cmdStr, "xpath(") && strings.HasSuffix(cmdStr, ")") {
		q := cmdStr[6 : len(cmdStr)-1]
		q = strings.Trim(q, "'\"")
		if val.RawNode == nil {
			return nil, fmt.Errorf("error executing xpath: input must be parsed xml")
		}

		nodes, err := xmlquery.QueryAll(val.RawNode, q)
		if err != nil {
			return nil, fmt.Errorf("error executing xpath: %v", err)
		}
		var results []interface{}
		for _, n := range nodes {
			text := n.InnerText()
			if text != "" {
				results = append(results, text)
			} else {
				href := n.SelectAttr("href")
				results = append(results, href)
			}
		}
		return &PipelineValue{List: results}, nil
	}

	if strings.HasPrefix(cmdStr, "regex(") && strings.HasSuffix(cmdStr, ")") {
		pattern := cmdStr[6 : len(cmdStr)-1]
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex %q: %v", pattern, err)
		}
		if val.IsList() {
			var results []interface{}
			for _, item := range val.List {
				m := re.FindStringSubmatch(fmt.Sprintf("%v", item))
				if m != nil {
					if len(m) > 1 {
						results = append(results, m[1])
					} else {
						results = append(results, m[0])
					}
				}
			}
			return &PipelineValue{List: results}, nil
		} else if !val.IsEmpty {
			m := re.FindStringSubmatch(val.GetString())
			if m != nil {
				if len(m) > 1 {
					return &PipelineValue{Value: m[1]}, nil
				}
				return &PipelineValue{Value: m[0]}, nil
			}
			return &PipelineValue{Value: ""}, nil
		}
		return val, nil
	}

	if strings.HasPrefix(cmdStr, "json(") && strings.HasSuffix(cmdStr, ")") {
		path := cmdStr[5 : len(cmdStr)-1]
		if val.IsEmpty || val.IsList() {
			return nil, fmt.Errorf("error parsing JSON path %s: input must be scalar string", path)
		}
		var obj interface{}
		strVal := val.GetString()
		err := json.Unmarshal([]byte(strVal), &obj)
		if err != nil {
			return nil, fmt.Errorf("error parsing JSON path %s: %v", path, err)
		}

		parts := strings.Split(path, ".")
		for _, part := range parts {
			if part == "" {
				continue
			}
			if m, ok := obj.(map[string]interface{}); ok {
				obj = m[part]
			} else if l, ok := obj.([]interface{}); ok {
				idx, err := strconv.Atoi(part)
				if err == nil && idx >= 0 && idx < len(l) {
					obj = l[idx]
				} else {
					obj = nil
				}
			} else {
				obj = nil
			}
		}

		if obj == nil {
			return &PipelineValue{IsEmpty: true}, nil
		}

		if v, ok := obj.([]interface{}); ok {
			return &PipelineValue{List: v, IsEmpty: false}, nil
		}

		// Primitives (string, bool, float64, map)
		if _, ok := obj.(map[string]interface{}); ok {
			b, _ := json.Marshal(obj)
			return &PipelineValue{Value: string(b), IsEmpty: false}, nil
		}
		return &PipelineValue{Value: obj, IsEmpty: false}, nil
	}

	if strings.HasPrefix(cmdStr, "replace(") && strings.HasSuffix(cmdStr, ")") {
		argsStr := cmdStr[8 : len(cmdStr)-1]
		args, err := parseReplaceArgs(argsStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing replace arguments: %w", err)
		}
		oldStr, newStr := args[0], args[1]

		if val.IsEmpty {
			return val, nil
		}
		if val.IsList() {
			var results []interface{}
			for _, item := range val.List {
				results = append(results, strings.ReplaceAll(fmt.Sprintf("%v", item), oldStr, newStr))
			}
			return &PipelineValue{List: results, IsEmpty: false}, nil
		}
		return &PipelineValue{Value: strings.ReplaceAll(val.GetString(), oldStr, newStr), IsEmpty: false}, nil
	}

	return nil, fmt.Errorf("unknown command: %s", cmdStr)
}

func parseReplaceArgs(argsStr string) ([]string, error) {
	rawArgs, err := splitReplaceArgs(argsStr)
	if err != nil {
		return nil, err
	}
	if len(rawArgs) != 2 {
		return nil, fmt.Errorf("replace requires exactly 2 arguments")
	}

	parsed := make([]string, 0, len(rawArgs))
	for _, raw := range rawArgs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil, fmt.Errorf("arguments must be strings")
		}
		unquoted, err := parseQuotedString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("arguments must be strings: %w", err)
		}
		parsed = append(parsed, unquoted)
	}
	return parsed, nil
}

func splitReplaceArgs(argsStr string) ([]string, error) {
	var args []string
	var current strings.Builder
	inQuote := rune(0)
	escapeNext := false

	for _, char := range argsStr {
		if escapeNext {
			current.WriteRune(char)
			escapeNext = false
			continue
		}
		if char == '\\' {
			escapeNext = true
			current.WriteRune(char)
			continue
		}
		switch inQuote {
		case 0:
			if char == '\'' || char == '"' {
				inQuote = char
				current.WriteRune(char)
				continue
			}
			if char == ',' {
				args = append(args, current.String())
				current.Reset()
				continue
			}
		default:
			if char == inQuote {
				inQuote = 0
			}
			current.WriteRune(char)
			continue
		}
		current.WriteRune(char)
	}

	if escapeNext {
		return nil, fmt.Errorf("dangling escape in replace arguments")
	}
	if inQuote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}

	args = append(args, current.String())
	return args, nil
}

func parseQuotedString(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	if len(arg) < 2 {
		return "", fmt.Errorf("argument must be a quoted string, got: %s", arg)
	}
	runes := []rune(arg)
	quote := runes[0]
	if quote != '\'' && quote != '"' {
		return "", fmt.Errorf("argument must be a quoted string, got: %s", arg)
	}

	var unescaped strings.Builder
	escapeNext := false
	closed := false

	for i := 1; i < len(runes); i++ {
		c := runes[i]
		if closed {
			return "", fmt.Errorf("argument has trailing characters after quote: %s", arg)
		}
		if escapeNext {
			switch c {
			case '\'', '"', '\\':
				unescaped.WriteRune(c)
			case 'n':
				unescaped.WriteRune('\n')
			case 't':
				unescaped.WriteRune('\t')
			case 'r':
				unescaped.WriteRune('\r')
			default:
				unescaped.WriteRune('\\')
				unescaped.WriteRune(c)
			}
			escapeNext = false
			continue
		}
		if c == '\\' {
			escapeNext = true
			continue
		}
		if c == quote {
			closed = true
			continue
		}
		unescaped.WriteRune(c)
	}

	if escapeNext {
		return "", fmt.Errorf("dangling escape in string: %s", arg)
	}
	if !closed {
		return "", fmt.Errorf("unterminated quote in string: %s", arg)
	}

	return unescaped.String(), nil
}
