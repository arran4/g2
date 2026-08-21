package main

import (
	"testing"
)

func TestIsCategoryAllowed(t *testing.T) {
	tests := []struct {
		name            string
		repoCats        map[string]bool
		hasGentooMaster bool
		mainCats        map[string]bool
		catName         string
		want            bool
	}{
		{
			name:            "category explicitly listed by the overlay -> included",
			repoCats:        map[string]bool{"sys-foo": true},
			hasGentooMaster: false,
			mainCats:        map[string]bool{},
			catName:         "sys-foo",
			want:            true,
		},
		{
			name:            "category not locally listed but present in Gentoo -> included when masters = gentoo",
			repoCats:        map[string]bool{"sys-foo": true},
			hasGentooMaster: true,
			mainCats:        map[string]bool{"dev-util": true},
			catName:         "dev-util",
			want:            true,
		},
		{
			name:            "the same Gentoo category -> not automatically inherited by a repository without masters = gentoo",
			repoCats:        map[string]bool{"sys-foo": true},
			hasGentooMaster: false,
			mainCats:        map[string]bool{"dev-util": true},
			catName:         "dev-util",
			want:            false,
		},
		{
			name:            "category absent from both allowed sets -> rejected",
			repoCats:        map[string]bool{"sys-foo": true},
			hasGentooMaster: true,
			mainCats:        map[string]bool{"dev-util": true},
			catName:         "dev-python",
			want:            false,
		},
		{
			name:            "virtual category -> continue to work",
			repoCats:        map[string]bool{"sys-foo": true},
			hasGentooMaster: false,
			mainCats:        map[string]bool{"dev-util": true},
			catName:         "virtual",
			want:            true,
		},
		{
			name:            "virtual-* category -> continue to work",
			repoCats:        map[string]bool{"sys-foo": true},
			hasGentooMaster: false,
			mainCats:        map[string]bool{"dev-util": true},
			catName:         "virtual-python",
			want:            true,
		},
		{
			name:            "unavailable/empty Gentoo category data does not cause arbitrary directories to be accepted",
			repoCats:        map[string]bool{"sys-foo": true},
			hasGentooMaster: true,
			mainCats:        map[string]bool{},
			catName:         "dev-util",
			want:            false,
		},
		{
			name:            "absent profiles/categories allows anything (fallback)",
			repoCats:        map[string]bool{},
			hasGentooMaster: true,
			mainCats:        map[string]bool{"dev-util": true},
			catName:         "random-dir",
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCategoryAllowed(tt.repoCats, tt.hasGentooMaster, tt.mainCats, tt.catName); got != tt.want {
				t.Errorf("isCategoryAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
