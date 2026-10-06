package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv reads KEY=VALUE pairs from the first .env file found (current directory,
// then its parent) into the process environment. Variables already set are kept, so
// real environment variables always win. A missing file is not an error.
func LoadDotEnv() (path string) {
	for _, p := range []string{".env", "../.env"} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			key, val, ok := strings.Cut(line, "=")
			key = strings.TrimSpace(key)
			if !ok || key == "" {
				continue
			}
			val = strings.TrimSpace(val)
			if n := len(val); n >= 2 && (val[0] == '"' && val[n-1] == '"' || val[0] == '\'' && val[n-1] == '\'') {
				val = val[1 : n-1]
			}
			if _, set := os.LookupEnv(key); !set {
				os.Setenv(key, val)
			}
		}
		return p
	}
	return ""
}
