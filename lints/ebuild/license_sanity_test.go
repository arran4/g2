package ebuild

import (
	"testing"

	"github.com/arran4/g2"
)

func TestLicenseSanityLintRule(t *testing.T) {
	rule := &LicenseSanityLintRule{}

	tests := []struct {
		name       string
		licenseStr string
		expectWarn bool
	}{
		{
			name:       "Normal license",
			licenseStr: "GPL-2",
			expectWarn: false,
		},
		{
			name:       "Multiple licenses",
			licenseStr: "GPL-2 MIT",
			expectWarn: false,
		},
		{
			name:       "Conditional licenses",
			licenseStr: "|| ( GPL-2 MIT )",
			expectWarn: false,
		},
		{
			name:       "Full text license short",
			licenseStr: "This is a full text license for the software.",
			expectWarn: true,
		},
		{
			name:       "Full text license long",
			licenseStr: "Allen Institute Software License  This software license is the 2-clause BSD license plus clause a third clause that prohibits redistribution and use for commercial purposes without further permission.  Copyright  2020. Allen Institute.  All rights reserved.  Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:  1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.  2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.  3. Redistributions and use for commercial purposes are not permitted without the Allen Institutes written permission. For purposes of this license, commercial purposes are the incorporation of the Allen Institute's software into anything for which you will charge fees or other compensation or use of the software to perform a commercial service for a third party. Contact terms@alleninstitute.org for commercial licensing opportunities.  THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS AS IS AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT INCLUDING NEGLIGENCE OR OTHERWISE ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.",
			expectWarn: true,
		},
		{
			name:       "Lots of punctuation",
			licenseStr: "Some, license, with, lots, of, commas",
			expectWarn: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkg := &g2.PackageData{
				Versions: []g2.VersionData{
					{
						Version: "1.0",
						Ebuild: &g2.Ebuild{
							Vars: map[string]string{
								"LICENSE": tc.licenseStr,
							},
						},
					},
				},
			}

			warnings := rule.Lint("", pkg)

			if tc.expectWarn && len(warnings) == 0 {
				t.Errorf("Expected a warning for license: %s, but got none", tc.licenseStr)
			}
			if !tc.expectWarn && len(warnings) > 0 {
				t.Errorf("Expected no warnings for license: %s, but got: %v", tc.licenseStr, warnings)
			}
		})
	}
}
