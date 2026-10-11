package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
)

// Structured asks for a JSON reply that validates against schema, retries once
// with the validation errors, and decodes the valid reply into out. inspect's
// disposition (WP4.8) and review lane A (WP6.6) use it.
func Structured(ctx context.Context, complete func(context.Context, Request) (Response, error), req Request, schema []byte, out any) error {
	var s Schema
	if err := json.Unmarshal(schema, &s); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	req.System = strings.TrimSpace(req.System + "\n\nReply with one JSON value and nothing else. It must validate against this JSON Schema:\n" + string(schema))
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := complete(ctx, req)
		if err != nil {
			return err
		}
		raw := extractJSON(resp.Text)
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			lastErr = fmt.Errorf("reply is not JSON: %w", err)
		} else if errs := s.Validate(v); len(errs) > 0 {
			lastErr = fmt.Errorf("reply does not validate: %s", strings.Join(errs, "; "))
		} else {
			return json.Unmarshal([]byte(raw), out)
		}
		req.Messages = append(req.Messages,
			Message{Role: "assistant", Content: resp.Text},
			Message{Role: "user", Content: "That reply was rejected: " + lastErr.Error() + ". Reply again with only the corrected JSON."})
	}
	return lastErr
}

var fence = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

// extractJSON strips a Markdown code fence or leading prose around one JSON value.
func extractJSON(s string) string {
	if m := fence.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "{["); i > 0 {
		s = s[i:]
	}
	return s
}

// Schema is the JSON Schema subset the structured helper validates: type,
// properties, required, additionalProperties (bool or schema), items, enum,
// const, minimum, maximum, minLength, maxLength, pattern, minItems, maxItems.
// No JSON Schema validator is on the plan's dependency list (§2), so this
// subset is implemented here (Appendix B D-1.6a). Unknown keywords are ignored.
type Schema struct {
	Type                 any                `json:"type"` // string or []string
	Properties           map[string]*Schema `json:"properties"`
	Required             []string           `json:"required"`
	AdditionalProperties json.RawMessage    `json:"additionalProperties"`
	Items                *Schema            `json:"items"`
	Enum                 []any              `json:"enum"`
	Const                any                `json:"const"`
	Minimum              *float64           `json:"minimum"`
	Maximum              *float64           `json:"maximum"`
	MinLength            *int               `json:"minLength"`
	MaxLength            *int               `json:"maxLength"`
	Pattern              string             `json:"pattern"`
	MinItems             *int               `json:"minItems"`
	MaxItems             *int               `json:"maxItems"`
}

// Validate returns one message per violation, with JSON-pointer-like paths.
func (s *Schema) Validate(v any) []string {
	var errs []string
	s.validate("$", v, &errs)
	return errs
}

func typeOf(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		if x == math.Trunc(x) {
			return "integer"
		}
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

func (s *Schema) types() []string {
	switch t := s.Type.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, x := range t {
			if str, ok := x.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

func (s *Schema) validate(path string, v any, errs *[]string) {
	bad := func(format string, args ...any) { *errs = append(*errs, path+": "+fmt.Sprintf(format, args...)) }
	if ts := s.types(); len(ts) > 0 {
		got := typeOf(v)
		ok := slices.Contains(ts, got) || (got == "integer" && slices.Contains(ts, "number"))
		if !ok {
			bad("want %s, got %s", strings.Join(ts, " or "), got)
			return
		}
	}
	if s.Enum != nil && !slices.ContainsFunc(s.Enum, func(e any) bool { return equalJSON(e, v) }) {
		bad("not one of the allowed values")
	}
	if s.Const != nil && !equalJSON(s.Const, v) {
		bad("must equal %v", s.Const)
	}
	switch x := v.(type) {
	case float64:
		if s.Minimum != nil && x < *s.Minimum {
			bad("%v is below the minimum %v", x, *s.Minimum)
		}
		if s.Maximum != nil && x > *s.Maximum {
			bad("%v is above the maximum %v", x, *s.Maximum)
		}
	case string:
		n := len([]rune(x))
		if s.MinLength != nil && n < *s.MinLength {
			bad("shorter than %d characters", *s.MinLength)
		}
		if s.MaxLength != nil && n > *s.MaxLength {
			bad("longer than %d characters", *s.MaxLength)
		}
		if s.Pattern != "" {
			if re, err := regexp.Compile(s.Pattern); err == nil && !re.MatchString(x) {
				bad("does not match %s", s.Pattern)
			}
		}
	case []any:
		if s.MinItems != nil && len(x) < *s.MinItems {
			bad("fewer than %d items", *s.MinItems)
		}
		if s.MaxItems != nil && len(x) > *s.MaxItems {
			bad("more than %d items", *s.MaxItems)
		}
		if s.Items != nil {
			for i, item := range x {
				s.Items.validate(fmt.Sprintf("%s[%d]", path, i), item, errs)
			}
		}
	case map[string]any:
		for _, r := range s.Required {
			if _, ok := x[r]; !ok {
				bad("missing required property %q", r)
			}
		}
		var extra *Schema
		allowExtra := true
		if len(s.AdditionalProperties) > 0 {
			var b bool
			if json.Unmarshal(s.AdditionalProperties, &b) == nil {
				allowExtra = b
			} else {
				extra = &Schema{}
				_ = json.Unmarshal(s.AdditionalProperties, extra)
			}
		}
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := path + "." + k
			switch p, ok := s.Properties[k]; {
			case ok:
				p.validate(child, x[k], errs)
			case extra != nil:
				extra.validate(child, x[k], errs)
			case !allowExtra:
				*errs = append(*errs, child+": property not allowed")
			}
		}
	}
}

func equalJSON(a, b any) bool {
	x, err1 := json.Marshal(a)
	y, err2 := json.Marshal(b)
	return errors.Join(err1, err2) == nil && string(x) == string(y)
}
