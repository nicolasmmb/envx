package envx

import (
	"strconv"
	"strings"
)

type envxTag struct {
	Name       string
	Required   bool
	Secret     bool
	Deprecated bool
	Default    string
	HasDefault bool
	Enum       []string
	Skip       bool
}

func parseEnvxTag(raw string) envxTag {
	tag := envxTag{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return tag
	}
	if raw == "-" {
		tag.Skip = true
		return tag
	}

	parts := splitEnvxTagParts(raw)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "-" {
			tag.Skip = true
			continue
		}

		key, val, ok := strings.Cut(part, "=")
		if !ok {
			switch strings.ToLower(part) {
			case "required":
				tag.Required = true
			case "secret":
				tag.Secret = true
			case "deprecated":
				tag.Deprecated = true
			}
			continue
		}

		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		val = unquoteEnvxTagValue(val)
		switch key {
		case "name":
			tag.Name = val
		case "required":
			if parsed, ok := parseEnvxBool(val); ok {
				tag.Required = parsed
			}
		case "secret":
			if parsed, ok := parseEnvxBool(val); ok {
				tag.Secret = parsed
			}
		case "deprecated":
			if parsed, ok := parseEnvxBool(val); ok {
				tag.Deprecated = parsed
			}
		case "default":
			tag.Default = val
			tag.HasDefault = true
		case "enum":
			tag.Enum = parseEnvxEnumList(val)
		}
	}

	return tag
}

func parseEnvxBool(val string) (bool, bool) {
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return false, false
	}
	return parsed, true
}

func unquoteEnvxTagValue(val string) string {
	if len(val) < 2 {
		return val
	}
	if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
		return val[1 : len(val)-1]
	}
	return val
}

func splitEnvxTagParts(raw string) []string {
	var parts []string
	var buf strings.Builder
	var quote rune

	for _, r := range raw {
		switch r {
		case ',':
			if quote == 0 {
				parts = append(parts, buf.String())
				buf.Reset()
				continue
			}
		case '"', '\'':
			if quote == 0 {
				quote = r
			} else if quote == r {
				quote = 0
			}
		}
		buf.WriteRune(r)
	}

	if buf.Len() > 0 || raw != "" {
		parts = append(parts, buf.String())
	}

	return parts
}

func parseEnvxEnumList(val string) []string {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}
	return items
}
