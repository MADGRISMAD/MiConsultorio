package config

import (
	"os"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	content := `# a comment line
T_PLAIN=value
T_COMMENT=lokoloko   # mínimo 8 caracteres
T_TAB=tabbed	# comment after a tab
T_GLUED=abc#123
T_DQUOTED="has # inside"   # trailing comment
T_SQUOTED='single # quoted'
T_EXPORT_PREFIX_TEST=1
export T_EXPORTED=yes
T_EMPTY=
T_ONLY_COMMENT=# nothing here
T_URL=postgres://u:p@localhost:5432/db?sslmode=disable
T_PRESET=from-file
`
	if err := os.WriteFile(dir+"/.env", []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	keys := []string{"T_PLAIN", "T_COMMENT", "T_TAB", "T_GLUED", "T_DQUOTED", "T_SQUOTED", "T_EXPORT_PREFIX_TEST", "T_EXPORTED", "T_EMPTY", "T_ONLY_COMMENT", "T_URL"}
	for _, k := range keys {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}
	t.Setenv("T_PRESET", "from-environment") // real environment variables win over the file

	if got := LoadDotEnv(); got != ".env" {
		t.Fatalf("loaded %q", got)
	}
	want := map[string]string{
		"T_PLAIN":              "value",
		"T_COMMENT":            "lokoloko",
		"T_TAB":                "tabbed",
		"T_GLUED":              "abc#123",
		"T_DQUOTED":            "has # inside",
		"T_SQUOTED":            "single # quoted",
		"T_EXPORT_PREFIX_TEST": "1",
		"T_EXPORTED":           "yes",
		"T_EMPTY":              "",
		"T_ONLY_COMMENT":       "",
		"T_URL":                "postgres://u:p@localhost:5432/db?sslmode=disable",
		"T_PRESET":             "from-environment",
	}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}
