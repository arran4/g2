package main

import (
	"reflect"
	"testing"
)

func TestParseLintQuery(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedQuery LintQuery
	}{
		{
			name:  "basic package",
			query: "app-misc/foo",
			expectedQuery: LintQuery{
				RepoPath: ".",
				Category: "app-misc",
				Package:  "foo",
			},
		},
		{
			name:  "version operator",
			query: ">=app-misc/foo-1.2.3",
			expectedQuery: LintQuery{
				RepoPath:  ".",
				Category:  "app-misc",
				Package:   "foo",
				VersionOp: ">=",
				Version:   "1.2.3",
			},
		},
		{
			name:  "wildcard version",
			query: "app-misc/foo-v3",
			expectedQuery: LintQuery{
				RepoPath:  ".",
				Category:  "app-misc",
				Package:   "foo",
				VWildcard: "3",
			},
		},
		{
			name:  "repo suffix with repo fallback",
			query: "app-misc/foo::guru",
			expectedQuery: LintQuery{
				RepoPath: "/var/db/repos/guru",
				Category: "app-misc",
				Package:  "foo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := parseLintQuery(tt.query, ".")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(*q, tt.expectedQuery) {
				t.Errorf("expected %+v, got %+v", tt.expectedQuery, *q)
			}
		})
	}
}
