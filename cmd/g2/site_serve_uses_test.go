package main

import (
	"github.com/arran4/g2"
	"reflect"
	"testing"
)

func TestParseIUSEFlagsFunc(t *testing.T) {
	tests := []struct {
		name     string
		iuse     string
		expected []ParsedIUSEFlag
	}{
		{
			name: "Simple flags",
			iuse: "foo bar baz",
			expected: []ParsedIUSEFlag{
				{Name: "foo", Conditional: false, ConditionStr: ""},
				{Name: "bar", Conditional: false, ConditionStr: ""},
				{Name: "baz", Conditional: false, ConditionStr: ""},
			},
		},
		{
			name: "With default states",
			iuse: "+foo -bar baz",
			expected: []ParsedIUSEFlag{
				{Name: "foo", Conditional: true, ConditionStr: "Default: Enabled (+)"},
				{Name: "bar", Conditional: true, ConditionStr: "Default: Disabled (-)"},
				{Name: "baz", Conditional: false, ConditionStr: ""},
			},
		},
		{
			name:     "With empty string",
			iuse:     "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseIUSEFlagsFunc(tt.iuse)
			if len(got) == 0 && len(tt.expected) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseIUSEFlagsFunc() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetRepoUseFlags(t *testing.T) {
	site := &SiteData{
		RepoName: "test-repo",
		Categories: []CategoryData{
			{
				Name: "app-misc",
				Packages: []PackageData{
					{
						Category: "app-misc",
						Name:     "foo",
						Versions: []VersionData{
							{
								Version: "1.0",
								Ebuild: &g2.Ebuild{
									Vars: map[string]string{
										"IUSE": "test-flag +enabled-flag",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	aggPackages := map[string]*AggPackage{
		"app-misc/foo": {
			Name:     "foo",
			Category: "app-misc",
		},
	}

	flags := getRepoUseFlags(site, aggPackages)

	if len(flags) != 2 {
		t.Fatalf("Expected 2 flags, got %d", len(flags))
	}

	flagNames := map[string]bool{}
	for _, f := range flags {
		flagNames[f.Name] = true
	}

	if !flagNames["test-flag"] {
		t.Errorf("Expected test-flag to be in the result")
	}
	if !flagNames["enabled-flag"] {
		t.Errorf("Expected enabled-flag to be in the result")
	}
}
