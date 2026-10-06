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
			val = parseValue(strings.TrimSpace(val))
			if _, set := os.LookupEnv(key); !set {
				os.Setenv(key, val)
			}
		}
		return p
	}
	return ""
}

// parseValue interprets what follows "KEY=":
//   - "double" or 'single' quoted values keep everything inside the quotes (even a '#');
//     anything after the closing quote, such as a comment, is ignored;
//   - unquoted values end at a '#' that follows whitespace, so
//     `PASSWORD=secret   # at least 8 characters` is just "secret";
//     a '#' glued to the text (`abc#123`) is part of the value.
func parseValue(v string) string {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return v[1 : 1+end]
		}
	}
	if strings.HasPrefix(v, "#") {
		return ""
	}
	for i := 1; i < len(v); i++ {
		if v[i] == '#' && (v[i-1] == ' ' || v[i-1] == '\t') {
			return strings.TrimSpace(v[:i])
		}
	}
	return v
}
