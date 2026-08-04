// Package redact provides helpers for scrubbing sensitive data (SAS tokens,
// account keys, passwords, etc.) from strings before they are written to logs
// or telemetry.
package redact

import (
	"regexp"
	"strings"
)

// Placeholder is substituted for secret values that have been redacted.
const Placeholder = "<redacted>"

// sensitiveKeywords is the alternation of substrings that mark a key/name as
// likely to carry a secret (e.g. SAS tokens, account keys, passwords). It is
// shared by the key-name and JSON-field matchers so the rules stay in sync.
const sensitiveKeywords = `sas|token|secret|password|passwd|pwd|credential|connection[_-]?string|account[_-]?key|api[_-]?key|access[_-]?key`

// sensitiveKeyPattern matches environment variable / attribute names that are
// likely to carry secrets (e.g. SAS tokens, account keys, passwords).
var sensitiveKeyPattern = regexp.MustCompile(`(?i)(` + sensitiveKeywords + `)`)

// jsonSensitiveFieldPattern matches a JSON string field whose key name looks
// sensitive, capturing the key (and separator) so the value can be replaced.
// Only string values are matched; numbers/booleans/objects are left untouched.
var jsonSensitiveFieldPattern = regexp.MustCompile(`(?i)("[^"]*(?:` + sensitiveKeywords + `)[^"]*"\s*:\s*)"(?:[^"\\]|\\.)*"`)

// urlQueryPattern matches the query-string portion of an http(s) URL, where SAS
// tokens and similar credentials are typically passed. The character classes stop
// at whitespace and quote/bracket delimiters so that URLs embedded in JSON or
// other structured text keep their surrounding syntax intact.
var urlQueryPattern = regexp.MustCompile(`(https?://[^\s?"'<>]+)\?[^\s"'<>]*`)

// Value scrubs a single string that may contain secrets before it is written to
// logs or telemetry. It redacts the value of a KEY=VALUE pair whose key looks
// sensitive, and strips query strings (which may hold SAS tokens) from any URLs.
func Value(s string) string {
	if idx := strings.Index(s, "="); idx > 0 && sensitiveKeyPattern.MatchString(s[:idx]) {
		return s[:idx+1] + Placeholder
	}
	return urlQueryPattern.ReplaceAllString(s, "$1?"+Placeholder)
}

// Slice returns a copy of values with sensitive data redacted so that command
// args/env can be safely logged. The input slice is not modified.
func Slice(values []string) []string {
	redacted := make([]string, len(values))
	for i, v := range values {
		redacted[i] = Value(v)
	}
	return redacted
}

// Text scrubs secrets from free-form, possibly multi-line text (such as captured
// process output or serialized settings) before it is logged. It strips query
// strings (which may contain SAS tokens) from every URL it contains. Unlike
// Value, it does not apply KEY=VALUE redaction, so it is safe to run over
// arbitrary text without corrupting unrelated content.
func Text(s string) string {
	return urlQueryPattern.ReplaceAllString(s, "$1?"+Placeholder)
}

// JSON scrubs secrets from a JSON document (such as serialized settings) before
// it is logged. It replaces the string value of any field whose key name looks
// sensitive with the placeholder, and strips query strings (SAS tokens) from any
// URLs. The result remains valid JSON.
func JSON(s string) string {
	s = jsonSensitiveFieldPattern.ReplaceAllString(s, `${1}"`+Placeholder+`"`)
	return urlQueryPattern.ReplaceAllString(s, "$1?"+Placeholder)
}
