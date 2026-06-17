package main

import (
	"reflect"
	"testing"
)

func TestParseEnv(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "empty",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "comments and blanks",
			input: "# top\n\n# another\n",
			want:  map[string]string{},
		},
		{
			name:  "basic",
			input: "FOO=bar\nBAZ=qux\n",
			want:  map[string]string{"FOO": "bar", "BAZ": "qux"},
		},
		{
			name:  "double quotes",
			input: `FOO="hello world"`,
			want:  map[string]string{"FOO": "hello world"},
		},
		{
			name:  "single quotes",
			input: `FOO='hello world'`,
			want:  map[string]string{"FOO": "hello world"},
		},
		{
			name:  "value with equals",
			input: "URL=a=b=c",
			want:  map[string]string{"URL": "a=b=c"},
		},
		{
			name:  "whitespace trimmed",
			input: "  FOO  =  bar  \n",
			want:  map[string]string{"FOO": "bar"},
		},
		{
			name:  "line without equals skipped",
			input: "JUST_A_KEY\nFOO=bar",
			want:  map[string]string{"FOO": "bar"},
		},
		{
			name:  "mixed",
			input: "# header\n\nDB_URL='postgres://u:p@h/d'\nEMPTY=\nKEY=val=ue\n",
			want:  map[string]string{
				"DB_URL": "postgres://u:p@h/d",
				"EMPTY":  "",
				"KEY":    "val=ue",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseEnv(tc.input)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseEnv(%q)\n got: %v\nwant: %v", tc.input, got, tc.want)
			}
		})
	}
}