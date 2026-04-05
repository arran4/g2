package g2

import (
	"reflect"
	"testing"
)

func TestParseRequiredUse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    RequiredUseNode
		wantErr bool
	}{
		{
			name:  "empty",
			input: "",
			want:  RequiredUseAllOf{},
		},
		{
			name:  "single flag",
			input: "foo",
			want:  RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "foo"}}},
		},
		{
			name:  "negated flag",
			input: "!foo",
			want:  RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "!foo"}}},
		},
		{
			name:  "all of",
			input: "foo bar",
			want: RequiredUseAllOf{
				Nodes: []RequiredUseNode{
					RequiredUseFlag{Name: "foo"},
					RequiredUseFlag{Name: "bar"},
				},
			},
		},
		{
			name:  "any of",
			input: "|| ( foo bar )",
			want: RequiredUseAnyOf{
				Nodes: []RequiredUseNode{
					RequiredUseFlag{Name: "foo"},
					RequiredUseFlag{Name: "bar"},
				},
			},
		},
		{
			name:  "exactly one of",
			input: "^^ ( foo bar )",
			want: RequiredUseExactlyOneOf{
				Nodes: []RequiredUseNode{
					RequiredUseFlag{Name: "foo"},
					RequiredUseFlag{Name: "bar"},
				},
			},
		},
		{
			name:  "at most one of",
			input: "?? ( foo bar )",
			want: RequiredUseAtMostOneOf{
				Nodes: []RequiredUseNode{
					RequiredUseFlag{Name: "foo"},
					RequiredUseFlag{Name: "bar"},
				},
			},
		},
		{
			name:  "conditional",
			input: "foo? ( bar )",
			want: RequiredUseConditional{
				Condition: "foo",
				Nodes: RequiredUseAllOf{
					Nodes: []RequiredUseNode{
						RequiredUseFlag{Name: "bar"},
					},
				},
			},
		},
		{
			name:  "negated conditional",
			input: "!foo? ( bar )",
			want: RequiredUseConditional{
				Condition: "!foo",
				Nodes: RequiredUseAllOf{
					Nodes: []RequiredUseNode{
						RequiredUseFlag{Name: "bar"},
					},
				},
			},
		},
		{
			name:  "complex nested",
			input: "foo || ( bar baz ) qux? ( !quux )",
			want: RequiredUseAllOf{
				Nodes: []RequiredUseNode{
					RequiredUseFlag{Name: "foo"},
					RequiredUseAnyOf{
						Nodes: []RequiredUseNode{
							RequiredUseFlag{Name: "bar"},
							RequiredUseFlag{Name: "baz"},
						},
					},
					RequiredUseConditional{
						Condition: "qux",
						Nodes: RequiredUseAllOf{
							Nodes: []RequiredUseNode{
								RequiredUseFlag{Name: "!quux"},
							},
						},
					},
				},
			},
		},
		// Error cases
		{
			name:    "missing opening paren any of",
			input:   "|| foo bar )",
			wantErr: true,
		},
		{
			name:    "missing closing paren any of",
			input:   "|| ( foo bar",
			wantErr: true,
		},
		{
			name:    "missing opening paren exactly one of",
			input:   "^^ foo bar )",
			wantErr: true,
		},
		{
			name:    "missing closing paren exactly one of",
			input:   "^^ ( foo bar",
			wantErr: true,
		},
		{
			name:    "missing opening paren at most one of",
			input:   "?? foo bar )",
			wantErr: true,
		},
		{
			name:    "missing closing paren at most one of",
			input:   "?? ( foo bar",
			wantErr: true,
		},
		{
			name:    "missing opening paren conditional",
			input:   "foo? bar )",
			wantErr: true,
		},
		{
			name:    "missing closing paren conditional",
			input:   "foo? ( bar",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRequiredUse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRequiredUse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseRequiredUse() got = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestRequiredUseEvaluate(t *testing.T) {
	tests := []struct {
		name    string
		node    RequiredUseNode
		context map[string]bool
		want    bool
	}{
		// Flag
		{"flag true", RequiredUseFlag{Name: "foo"}, map[string]bool{"foo": true}, true},
		{"flag false", RequiredUseFlag{Name: "foo"}, map[string]bool{"foo": false}, false},
		{"flag missing", RequiredUseFlag{Name: "foo"}, map[string]bool{}, false},
		{"negated flag true", RequiredUseFlag{Name: "!foo"}, map[string]bool{"foo": false}, true},
		{"negated flag false", RequiredUseFlag{Name: "!foo"}, map[string]bool{"foo": true}, false},
		{"negated flag missing", RequiredUseFlag{Name: "!foo"}, map[string]bool{}, true},

		// AllOf
		{"all of empty", RequiredUseAllOf{}, map[string]bool{}, true},
		{"all of true", RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": true, "b": true}, true},
		{"all of false", RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": true, "b": false}, false},

		// AnyOf
		{"any of empty", RequiredUseAnyOf{}, map[string]bool{}, false},
		{"any of true", RequiredUseAnyOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": true, "b": false}, true},
		{"any of both false", RequiredUseAnyOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": false, "b": false}, false},

		// ExactlyOneOf
		{"exactly one of true", RequiredUseExactlyOneOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": true, "b": false}, true},
		{"exactly one of both true", RequiredUseExactlyOneOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": true, "b": true}, false},
		{"exactly one of both false", RequiredUseExactlyOneOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": false, "b": false}, false},

		// AtMostOneOf
		{"at most one of none", RequiredUseAtMostOneOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": false, "b": false}, true},
		{"at most one of one", RequiredUseAtMostOneOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": true, "b": false}, true},
		{"at most one of both", RequiredUseAtMostOneOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, map[string]bool{"a": true, "b": true}, false},

		// Conditional
		{"conditional met and true", RequiredUseConditional{Condition: "cond", Nodes: RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}}}}, map[string]bool{"cond": true, "a": true}, true},
		{"conditional met and false", RequiredUseConditional{Condition: "cond", Nodes: RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}}}}, map[string]bool{"cond": true, "a": false}, false},
		{"conditional not met", RequiredUseConditional{Condition: "cond", Nodes: RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}}}}, map[string]bool{"cond": false, "a": false}, true},
		{"negated conditional met and true", RequiredUseConditional{Condition: "!cond", Nodes: RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}}}}, map[string]bool{"cond": false, "a": true}, true},
		{"negated conditional met and false", RequiredUseConditional{Condition: "!cond", Nodes: RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}}}}, map[string]bool{"cond": false, "a": false}, false},
		{"negated conditional not met", RequiredUseConditional{Condition: "!cond", Nodes: RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}}}}, map[string]bool{"cond": true, "a": false}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.Evaluate(tt.context); got != tt.want {
				t.Errorf("Evaluate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequiredUseString(t *testing.T) {
	tests := []struct {
		name string
		node RequiredUseNode
		want string
	}{
		{"flag", RequiredUseFlag{Name: "foo"}, "foo"},
		{"all of", RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, "a b"},
		{"any of", RequiredUseAnyOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, "|| ( a b )"},
		{"exactly one of", RequiredUseExactlyOneOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, "^^ ( a b )"},
		{"at most one of", RequiredUseAtMostOneOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}, RequiredUseFlag{Name: "b"}}}, "?? ( a b )"},
		{"conditional", RequiredUseConditional{Condition: "cond", Nodes: RequiredUseAllOf{Nodes: []RequiredUseNode{RequiredUseFlag{Name: "a"}}}}, "cond? ( a )"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}
