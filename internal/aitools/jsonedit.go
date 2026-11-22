package aitools

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// node is a JSON value that remembers the order of object keys, so a file
// ATLAS edits keeps its layout apart from the one entry that changed.
type node struct {
	kind    byte // 'o' object, 'a' array, 's' scalar
	members []member
	items   []*node
	raw     string // scalar, already encoded
}

type member struct {
	key string
	val *node
}

func parseJSON(data []byte) (*node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	n, err := parseNode(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the end")
	}
	return n, nil
}

func parseNode(dec *json.Decoder) (*node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch v := tok.(type) {
	case json.Delim:
		switch v {
		case '{':
			n := &node{kind: 'o'}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, ok := kt.(string)
				if !ok {
					return nil, errors.New("object key is not a string")
				}
				val, err := parseNode(dec)
				if err != nil {
					return nil, err
				}
				n.members = append(n.members, member{key, val})
			}
			_, err := dec.Token() // }
			return n, err
		case '[':
			n := &node{kind: 'a'}
			for dec.More() {
				val, err := parseNode(dec)
				if err != nil {
					return nil, err
				}
				n.items = append(n.items, val)
			}
			_, err := dec.Token() // ]
			return n, err
		}
		return nil, fmt.Errorf("unexpected %v", v)
	case json.Number:
		return &node{kind: 's', raw: v.String()}, nil
	case nil:
		return &node{kind: 's', raw: "null"}, nil
	default:
		return &node{kind: 's', raw: encodeScalar(v)}, nil
	}
}

func encodeScalar(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

// get returns the member value for key, or nil.
func (n *node) get(key string) *node {
	if n == nil || n.kind != 'o' {
		return nil
	}
	for _, m := range n.members {
		if m.key == key {
			return m.val
		}
	}
	return nil
}

// at follows a key path.
func (n *node) at(path ...string) *node {
	for _, k := range path {
		n = n.get(k)
	}
	return n
}

// remove deletes key from an object and reports whether it was there.
func (n *node) remove(key string) bool {
	if n == nil || n.kind != 'o' {
		return false
	}
	for i, m := range n.members {
		if m.key == key {
			n.members = append(n.members[:i], n.members[i+1:]...)
			return true
		}
	}
	return false
}

// set puts key = val, replacing an existing member in place or appending.
func (n *node) set(key string, val *node) {
	for i, m := range n.members {
		if m.key == key {
			n.members[i].val = val
			return
		}
	}
	n.members = append(n.members, member{key, val})
}

// str is the value of a string scalar, or "".
func (n *node) str() string {
	if n == nil || n.kind != 's' || !strings.HasPrefix(n.raw, `"`) {
		return ""
	}
	var s string
	_ = json.Unmarshal([]byte(n.raw), &s)
	return s
}

// boolean reports a true/false scalar; ok is false for anything else.
func (n *node) boolean() (v, ok bool) {
	if n == nil || n.kind != 's' {
		return false, false
	}
	switch n.raw {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

func boolNode(v bool) *node {
	if v {
		return &node{kind: 's', raw: "true"}
	}
	return &node{kind: 's', raw: "false"}
}

// encode writes n indented by two spaces, the layout these apps use.
func (n *node) encode() []byte {
	var b bytes.Buffer
	n.write(&b, 0)
	b.WriteByte('\n')
	return b.Bytes()
}

func (n *node) write(b *bytes.Buffer, depth int) {
	pad := func(d int) { b.WriteString(strings.Repeat("  ", d)) }
	switch n.kind {
	case 'o':
		if len(n.members) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteString("{\n")
		for i, m := range n.members {
			pad(depth + 1)
			b.WriteString(encodeScalar(m.key) + ": ")
			m.val.write(b, depth+1)
			if i < len(n.members)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		pad(depth)
		b.WriteByte('}')
	case 'a':
		if len(n.items) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, it := range n.items {
			pad(depth + 1)
			it.write(b, depth+1)
			if i < len(n.items)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		pad(depth)
		b.WriteByte(']')
	default:
		b.WriteString(n.raw)
	}
}

// stripJSONC removes // and /* */ comments and trailing commas, which VS
// Code's settings files allow, so they can be read. Strings are left alone.
func stripJSONC(data []byte) []byte {
	var out []byte
	inStr, esc := false, false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inStr {
			out = append(out, c)
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch {
		case c == '"':
			inStr = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && (data[i] != '*' || data[i+1] != '/') {
				i++
			}
			i++
		case c == '}' || c == ']':
			// Drop a trailing comma before the closing bracket.
			j := len(out) - 1
			for j >= 0 && (out[j] == ' ' || out[j] == '\t' || out[j] == '\n' || out[j] == '\r') {
				j--
			}
			if j >= 0 && out[j] == ',' {
				out = append(out[:j], out[j+1:]...)
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}
