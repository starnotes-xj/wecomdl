package redact

import (
	"net/url"
	"regexp"
	"strings"
)

const placeholder = "<redacted>"

var sensitiveQueryKeys = map[string]struct{}{
	"access_token": {},
	"auth":         {},
	"code":         {},
	"key":          {},
	"oss_vcode":    {},
	"session":      {},
	"sessionid":    {},
	"sign":         {},
	"signature":    {},
	"ticket":       {},
	"token":        {},
}

var headerLinePattern = regexp.MustCompile(`(?im)^([A-Za-z0-9_-]*(?:cookie|authorization|token|session|secret|ticket|key|sign)[A-Za-z0-9_-]*\s*:\s*).*$`)

func URL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := parsed.Query()
	for key := range query {
		if IsSensitiveKey(key) {
			query.Set(key, placeholder)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func Text(text string) string {
	text = headerLinePattern.ReplaceAllString(text, "${1}"+placeholder)
	return redactURLsInText(text)
}

func Args(args []string) []string {
	redacted := append([]string(nil), args...)
	for i := 0; i < len(redacted); i++ {
		switch redacted[i] {
		case "-i", "-referer":
			if i+1 < len(redacted) {
				redacted[i+1] = URL(redacted[i+1])
				i++
			}
		case "-headers":
			if i+1 < len(redacted) {
				redacted[i+1] = Text(redacted[i+1])
				i++
			}
		}
	}
	return redacted
}

func IsSensitiveKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	if _, ok := sensitiveQueryKeys[lower]; ok {
		return true
	}
	return strings.Contains(lower, "token") || strings.Contains(lower, "session") || strings.Contains(lower, "secret") || strings.Contains(lower, "ticket") || strings.Contains(lower, "signature")
}

func redactURLsInText(text string) string {
	fields := strings.Fields(text)
	for _, field := range fields {
		trimmed := strings.Trim(field, "'\"()[]{}<>,")
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			text = strings.ReplaceAll(text, trimmed, URL(trimmed))
		}
	}
	return text
}
