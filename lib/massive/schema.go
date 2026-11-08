package massive

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"slices"
	"strconv"
	"strings"
)

// schemaJSON is Massive's aggregates response as a JSON Schema, checked against Massive's published spec
// (aggregates.schema.json, which cites it).
//
//go:embed aggregates.schema.json
var schemaJSON []byte

// Schema gives the JSON Schema a page is checked against, its bytes as the file holds them.
func Schema() []byte { return slices.Clone(schemaJSON) }

// pageSchema is the schema compiled, once.
var pageSchema = func() *schema {
	s, err := compile(schemaJSON)
	if err != nil {
		panic(fmt.Sprintf("massive: aggregates.schema.json: %v", err))
	}
	return s
}()

// CheckPage checks a page against the schema: its JSON, a name at most once in each object, and each value of the
// type the schema gives it, every member the schema requires present.
func CheckPage(b []byte) error {
	_, err := checked(b)
	return err
}

// checked is a page decoded and checked against the schema: its objects map[string]any by their names exactly as
// written, its arrays []any, its numbers json.Number.
func checked(b []byte) (any, error) {
	v, err := decode(b)
	if err != nil {
		return nil, fmt.Errorf("not JSON: %w", err)
	}
	return v, pageSchema.check("", v)
}

// schema is a JSON Schema (2020-12) of the keywords this check knows: type, properties, required and items, and the
// annotations $schema, $id, $comment, title and description. A schema with any other keyword is refused, so that no
// keyword of it goes unchecked.
type schema struct {
	types      []string
	properties map[string]*schema
	required   []string
	items      *schema
}

var annotations = []string{"$schema", "$id", "$comment", "title", "description"}

var typeNames = []string{"object", "array", "string", "number", "integer", "boolean", "null"}

func compile(b []byte) (*schema, error) {
	v, err := decode(b)
	if err != nil {
		return nil, err
	}
	return compileAt("", v)
}

func compileAt(at string, v any) (*schema, error) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: a schema is an object", where(at))
	}
	s := &schema{}
	for k, kv := range m {
		var err error
		switch {
		case slices.Contains(annotations, k):
			if _, ok := kv.(string); !ok {
				err = fmt.Errorf("%s: %s: a string", where(at), k)
			}
		case k == "type":
			s.types, err = typesOf(kv)
		case k == "properties":
			pm, ok := kv.(map[string]any)
			if !ok {
				err = fmt.Errorf("properties: an object")
				break
			}
			s.properties = map[string]*schema{}
			for name, pv := range pm {
				if s.properties[name], err = compileAt(at+"/properties/"+name, pv); err != nil {
					return nil, err
				}
			}
		case k == "required":
			l, ok := kv.([]any)
			for _, r := range l {
				name, isString := r.(string)
				if !isString || slices.Contains(s.required, name) {
					ok = false
					break
				}
				s.required = append(s.required, name)
			}
			if !ok {
				err = fmt.Errorf("required: an array of names, each once")
			}
		case k == "items":
			s.items, err = compileAt(at+"/items", kv)
		default:
			err = fmt.Errorf("%q: a keyword this check does not know", k)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", where(at), err)
		}
	}
	slices.Sort(s.required)
	return s, nil
}

func typesOf(v any) ([]string, error) {
	var l []any
	switch v := v.(type) {
	case string:
		l = []any{v}
	case []any:
		l = v
	}
	var out []string
	for _, t := range l {
		name, ok := t.(string)
		if !ok || !slices.Contains(typeNames, name) || slices.Contains(out, name) {
			return nil, fmt.Errorf("type: %v: one of %s, or an array of them, each once", v, strings.Join(typeNames, ", "))
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("type: %v: no type", v)
	}
	return out, nil
}

// check checks a value at a JSON pointer against the schema.
func (s *schema) check(at string, v any) error {
	if s.types != nil && !slices.ContainsFunc(s.types, func(t string) bool { return is(t, v) }) {
		return fmt.Errorf("%s: %s, not %s", where(at), kind(v), strings.Join(s.types, " or "))
	}
	if m, ok := v.(map[string]any); ok {
		for _, name := range s.required {
			if _, ok := m[name]; !ok {
				return fmt.Errorf("%s: no %s", where(at), name)
			}
		}
		names := make([]string, 0, len(m))
		for name := range m {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if p, ok := s.properties[name]; ok {
				if err := p.check(at+"/"+pointer(name), m[name]); err != nil {
					return err
				}
			}
		}
	}
	if l, ok := v.([]any); ok && s.items != nil {
		for i, e := range l {
			if err := s.items.check(at+"/"+strconv.Itoa(i), e); err != nil {
				return err
			}
		}
	}
	return nil
}

// is reports whether a decoded value is of a JSON Schema type; an integer is a number of no fraction (1.0 among
// them), as JSON Schema has it, decided exactly from its digits.
func is(t string, v any) bool {
	switch v := v.(type) {
	case map[string]any:
		return t == "object"
	case []any:
		return t == "array"
	case string:
		return t == "string"
	case bool:
		return t == "boolean"
	case nil:
		return t == "null"
	case json.Number:
		if t == "number" {
			return true
		}
		r, ok := new(big.Rat).SetString(v.String())
		return t == "integer" && ok && r.IsInt()
	}
	return false
}

func kind(v any) string {
	switch v := v.(type) {
	case map[string]any:
		return "an object"
	case []any:
		return "an array"
	case string:
		return "a string"
	case bool:
		return "a boolean"
	case nil:
		return "null"
	case json.Number:
		return "the number " + v.String()
	}
	return fmt.Sprintf("%T", v)
}

func where(at string) string {
	if at == "" {
		return "the page"
	}
	return at
}

// pointer escapes a name for a JSON pointer (RFC 6901).
func pointer(name string) string {
	return strings.NewReplacer("~", "~0", "/", "~1").Replace(name)
}

// decode reads one JSON value, its numbers as their digits (json.Number), refusing a name twice in one object, by
// which a value would be lost, and anything after the value.
func decode(b []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	v, err := value(d)
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, errors.New("more after the JSON value")
	}
	return v, nil
}

func value(d *json.Decoder) (any, error) {
	t, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch t {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, err
			}
			name := k.(string)
			if _, ok := m[name]; ok {
				return nil, fmt.Errorf("%q twice in one object", name)
			}
			if m[name], err = value(d); err != nil {
				return nil, err
			}
		}
		_, err := d.Token()
		return m, err
	case json.Delim('['):
		l := []any{}
		for d.More() {
			e, err := value(d)
			if err != nil {
				return nil, err
			}
			l = append(l, e)
		}
		_, err := d.Token()
		return l, err
	}
	return t, nil
}
