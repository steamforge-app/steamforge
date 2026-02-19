package keyvalue

import (
	"fmt"
	"strconv"
)

// Type represents the type of a KeyValue node.
type Type byte

const (
	TypeNone       Type = 0x00 // Container node with children
	TypeString     Type = 0x01 // UTF-8 string
	TypeInt32      Type = 0x02 // int32
	TypeFloat32    Type = 0x03 // float32
	TypePointer    Type = 0x04 // uint32 pointer
	TypeWideString Type = 0x05 // UTF-16 string
	TypeColor      Type = 0x06 // uint32 color
	TypeUInt64     Type = 0x07 // uint64
	TypeEnd        Type = 0x08 // End of container marker
)

// Node represents a single key-value pair in the Valve KeyValue format.
type Node struct {
	Name     string
	Type     Type
	Value    interface{}
	Valid    bool
	Children []*Node
}

// Get returns a child node by name (case-insensitive).
// Returns an empty node if not found (never nil).
func (n *Node) Get(name string) *Node {
	if n == nil {
		return &Node{}
	}
	for _, child := range n.Children {
		if equalFold(child.Name, name) {
			return child
		}
	}
	return &Node{}
}

// AsString returns the value as a string, or the default.
func (n *Node) AsString(defaultValue string) string {
	if n == nil || !n.Valid {
		return defaultValue
	}
	switch v := n.Value.(type) {
	case string:
		return v
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	default:
		return defaultValue
	}
}

// AsInt returns the value as an int, or the default.
func (n *Node) AsInt(defaultValue int) int {
	if n == nil || !n.Valid {
		return defaultValue
	}
	switch v := n.Value.(type) {
	case int32:
		return int(v)
	case uint64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		return defaultValue
	default:
		return defaultValue
	}
}

// AsFloat returns the value as a float32, or the default.
func (n *Node) AsFloat(defaultValue float32) float32 {
	if n == nil || !n.Valid {
		return defaultValue
	}
	switch v := n.Value.(type) {
	case float32:
		return v
	case int32:
		return float32(v)
	case uint64:
		return float32(v)
	case string:
		if f, err := strconv.ParseFloat(v, 32); err == nil {
			return float32(f)
		}
		return defaultValue
	default:
		return defaultValue
	}
}

// AsBool returns the value as a bool, or the default.
// Zero values are false, nonzero are true.
func (n *Node) AsBool(defaultValue bool) bool {
	if n == nil || !n.Valid {
		return defaultValue
	}
	switch v := n.Value.(type) {
	case int32:
		return v != 0
	case uint64:
		return v != 0
	case float32:
		return v != 0
	case string:
		if i, err := strconv.Atoi(v); err == nil {
			return i != 0
		}
		return defaultValue
	default:
		return defaultValue
	}
}

// String returns a debug representation of the node.
func (n *Node) String() string {
	if n == nil {
		return "<nil>"
	}
	if n.Type == TypeNone {
		return fmt.Sprintf("{%s: %d children}", n.Name, len(n.Children))
	}
	return fmt.Sprintf("{%s: %v}", n.Name, n.Value)
}

// equalFold is a simple case-insensitive string comparison.
func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
