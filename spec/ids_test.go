// SPDX-License-Identifier: Apache-2.0

package spec

import "testing"

func TestParseID(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantVer  string
		wantErr  bool
	}{
		{"provider.anthropic@1.0.0", "provider.anthropic", "1.0.0", false},
		{"toolpack.http-get@2.3.4", "toolpack.http-get", "2.3.4", false},
		{"bad", "", "", true},
		{"nope@", "", "", true},
		{"@1.0.0", "", "", true},
		{"Foo@1.0.0", "", "", true}, // uppercase rejected
		{"foo@1", "", "", true},     // non-semver rejected
		{"foo@1.2", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			name, ver, err := ParseID(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, c.wantErr)
			}
			if name != c.wantName || ver != c.wantVer {
				t.Fatalf("got (%q,%q) want (%q,%q)", name, ver, c.wantName, c.wantVer)
			}
		})
	}
}
