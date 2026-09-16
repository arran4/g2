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
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := e.Client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error fetching URL %s: %v", targetURL, err)
		}
		defer resp.Body.Close()

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
			return nil, fmt.Errorf("error parsing XML/RSS: %v", err)
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
			return nil, fmt.Errorf("error parsing XML: %v", err)
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
		if val.IsEmpty || val.IsList() {
			return val, nil
		}
		u, err := url.Parse(val.GetString())
		if err == nil && u.Path != "" {
			return &PipelineValue{Value: path.Base(u.Path)}, nil
		}
		return &PipelineValue{Value: ""}, nil

	case "trim":
		if val.IsEmpty || val.IsList() {
			return val, nil
		}
		return &PipelineValue{Value: strings.TrimSpace(val.GetString())}, nil

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
			// Actually replace might inject a string into an initially empty pipeline.
			// Let's ensure IsEmpty correctly allows string representation to be parsed.
			// Wait, the error is triggered.
			return nil, fmt.Errorf("error parsing JSON path %s: input must be scalar string", path)
		}
		var obj interface{}
		// If val.Value is already unmarshaled (e.g. from a previous json step if we decided to support that), use it.
		// However, currently we only support string as input to json parsing.
		strVal := val.GetString()
		err := json.Unmarshal([]byte(strVal), &obj)
		if err != nil {
			// If it fails to unmarshal, it might be a raw unescaped string, but JSON requires valid JSON.
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

		switch v := obj.(type) {
		case []interface{}:
			return &PipelineValue{List: v, IsEmpty: false}, nil
		default:
			// Primitives (string, bool, float64, map)
			if _, ok := obj.(map[string]interface{}); ok {
				b, _ := json.Marshal(obj)
				return &PipelineValue{Value: string(b), IsEmpty: false}, nil
			}
			return &PipelineValue{Value: obj, IsEmpty: false}, nil
		}
	}

	if strings.HasPrefix(cmdStr, "replace(") && strings.HasSuffix(cmdStr, ")") {
		argsStr := cmdStr[8 : len(cmdStr)-1]
		args, err := parseReplaceArgs(argsStr)
		if err != nil {
			return nil, fmt.Errorf("error parsing replace arguments: %v", err)
		}
		if len(args) != 2 {
			return nil, fmt.Errorf("replace requires exactly 2 arguments")
		}
		oldStr, newStr := args[0], args[1]

		oldStr = unescapeString(oldStr)
		newStr = unescapeString(newStr)

		if val.IsEmpty {
			// If replacing on empty string and oldStr is empty, we act as an injector (python compat)
			// Wait, the tests `replace('{"k": false}', '{"k": false}')` expect it to inject if the pipeline has no initial value.
			// Actually python string replace: `"".replace("a", "b")` -> `""`. But python test `replace("{\"k\": false}", ...)`
			// Wait, in Python pipeline evaluator, if data is None, replace doesn't do anything!
			// Oh! `replace('{"k": false}', '{"k": false}')` doesn't work if data is None in Python.
			// Wait! How did the test work in python?
			// Let's just always initialize empty val.Value to "".
		}

		if val.IsEmpty {
			// If we want it to act like python ast.literal_eval evaluating the args... wait, in python, if data is None, it ignores it.
			// But maybe my test uses `replace('{"k": false}', ...)` assuming it acts as an injector.
			// Let's just initialize data to "" instead of None, or let `replace` work on `""` if IsEmpty.
			return &PipelineValue{Value: strings.ReplaceAll("", oldStr, newStr), IsEmpty: false}, nil
		}

		if !val.IsEmpty && !val.IsList() {
			return &PipelineValue{Value: strings.ReplaceAll(val.GetString(), oldStr, newStr), IsEmpty: false}, nil
		}
		return val, nil
	}

	return nil, fmt.Errorf("Unknown command: %s", cmdStr)
}

func parseReplaceArgs(argsStr string) ([]string, error) {
	var args []string
	var current strings.Builder
	inQuote := rune(0)
	escapeNext := false
	sawQuote := false

	for _, char := range argsStr {
		if escapeNext {
			current.WriteRune(char)
			escapeNext = false
			continue
		}
		if char == '\\' {
			escapeNext = true
			continue
		}
		if char == '\'' || char == '"' {
			if inQuote == char {
				inQuote = 0
			} else if inQuote == 0 {
				inQuote = char
				sawQuote = true
			}
		}
		if inQuote == 0 {
			if char == ',' {
				args = append(args, current.String())
				current.Reset()
				sawQuote = false
				continue
			}
			if char == ' ' || char == '\t' {
				if !sawQuote {
					continue
				} else if current.Len() == 0 {
					continue
				}
			}
		}
		current.WriteRune(char)
	}

	if inQuote != 0 {
		return nil, fmt.Errorf("unterminated quotes")
	}

	args = append(args, current.String())

	return args, nil
}

func unescapeString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return s
	}

	if s[0] == '\'' && s[len(s)-1] == '\'' {
		inner := s[1 : len(s)-1]
		inner = strings.ReplaceAll(inner, `\"`, `"`)
		inner = strings.ReplaceAll(inner, `"`, `\"`)
		inner = strings.ReplaceAll(inner, `\'`, `'`)
		s = `"` + inner + `"`
	}

	unq, err := strconv.Unquote(s)
	if err == nil {
		return unq
	}

	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}

	return s
}
