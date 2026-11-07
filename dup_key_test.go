package commentable

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/nicolasbonnici/gorest/serializer"
)

// TestResponseDTO_NoDuplicateCommentableKey guards against the regression where
// the response carried two "commentable" members: the literal type field and the
// FK relation IRI the JSON-LD serializer derives from commentableId. The type
// discriminator is now serialized as "commentableType".
func TestResponseDTO_NoDuplicateCommentableKey(t *testing.T) {
	dto := CommentResponseDTO{
		ID:              "3f1a7c22-5f6e-4a0b-9d3e-1c2b4a5d6e7f",
		CommentableID:   "a1b2c3d4-0000-0000-0000-000000000000",
		CommentableType: "post",
		Content:         "hello",
		Status:          "published",
	}

	raw, err := serializer.GetSerializer("jsonld").Serialize(dto, "/comments/"+dto.ID)
	if err != nil {
		t.Fatalf("serialize: %v", err)
	}

	keyCounts := countObjectKeys(t, raw)

	if keyCounts["commentable"] > 1 {
		t.Fatalf("duplicate \"commentable\" key in response: %d occurrences\n%s", keyCounts["commentable"], raw)
	}
	if keyCounts["commentableType"] != 1 {
		t.Fatalf("expected exactly one \"commentableType\" key, got %d\n%s", keyCounts["commentableType"], raw)
	}

	// The response must also be valid JSON (no duplicate members anywhere).
	var sink map[string]any
	if err := json.Unmarshal(raw, &sink); err != nil {
		t.Fatalf("response is not valid JSON: %v\n%s", err, raw)
	}
}

// countObjectKeys streams the JSON and counts how often each object key appears.
func countObjectKeys(t *testing.T, raw []byte) map[string]int {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	counts := map[string]int{}

	// Track whether the next string token is a key. In a JSON object, tokens
	// alternate key, value, key, value...; json.Decoder emits delimiters for
	// object/array boundaries which lets us know when we are reading object keys.
	type frame struct {
		isObject  bool
		expectKey bool
	}
	var stack []frame

	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				stack = append(stack, frame{isObject: true, expectKey: true})
			case '[':
				stack = append(stack, frame{isObject: false})
			case '}', ']':
				if len(stack) > 0 {
					stack = stack[:len(stack)-1]
				}
			}
		default:
			if len(stack) > 0 {
				top := &stack[len(stack)-1]
				if top.isObject && top.expectKey {
					if key, ok := v.(string); ok {
						counts[key]++
					}
					top.expectKey = false
				} else if top.isObject {
					top.expectKey = true
				}
			}
		}
	}
	return counts
}
