// Module 12, example 3: encoding/json — Marshal/Unmarshal, struct tags,
// omitempty, and handling JSON whose shape you don't know in advance.
//
// Run with: go run 03_json.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Struct tags control the JSON mapping.
// RULES:
//   - Only EXPORTED (capitalized) fields are (un)marshalled.
//   - `json:"name"`        → rename the key
//   - `json:"...,omitempty"` → drop the field when it's the zero value
//   - `json:"-"`           → never include this field
type User struct {
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"` // omitted when ""
	Age       int       `json:"age,omitempty"`   // omitted when 0 — careful if 0 is meaningful!
	CreatedAt time.Time `json:"created_at"`      // time.Time encodes as RFC3339
	Password  string    `json:"-"`               // secrets stay out of the payload
	internal  string    // unexported → invisible to encoding/json
}

func main() {
	_ = User{internal: "x"} // (silence "unused field" linters in this demo)

	// ----------------------- Marshal: Go → JSON -------------------------
	users := []User{
		{Name: "Ada", Email: "ada@example.com", Age: 36,
			CreatedAt: time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
			Password:  "s3cret"},
		{Name: "Linus", // Email/Age zero → omitempty removes them
			CreatedAt: time.Date(2026, 1, 15, 8, 30, 0, 0, time.UTC)},
	}

	compact, err := json.Marshal(users)
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal:", err)
		os.Exit(1)
	}
	fmt.Println("--- compact ---")
	fmt.Println(string(compact))

	// MarshalIndent for human-readable output. Note: no "password" key at
	// all, and Linus has no "email"/"age" thanks to omitempty.
	pretty, _ := json.MarshalIndent(users, "", "  ")
	fmt.Println("--- indented ---")
	fmt.Println(string(pretty))

	// ---------------------- Unmarshal: JSON → Go ------------------------
	incoming := `{
		"name": "Grace",
		"email": "grace@navy.mil",
		"age": 85,
		"created_at": "2026-03-01T12:00:00Z",
		"rank": "Rear Admiral"
	}`

	var u User
	// Pass a POINTER — Unmarshal fills your value in place.
	if err := json.Unmarshal([]byte(incoming), &u); err != nil {
		fmt.Fprintln(os.Stderr, "unmarshal:", err)
		os.Exit(1)
	}
	// Unknown keys ("rank") are silently IGNORED by default — often handy,
	// sometimes dangerous. json.Decoder's DisallowUnknownFields() makes
	// them an error instead.
	fmt.Printf("--- unmarshalled struct ---\n%+v\n", u)

	// ------------- Unknown shape: decode into map[string]any ------------
	// When you can't (or won't) define a struct, decode into map[string]any
	// and type-assert your way in.
	blob := `{
		"service": "billing",
		"replicas": 3,
		"healthy": true,
		"ports": [8080, 9090],
		"labels": {"team": "payments"}
	}`

	var doc map[string]any
	if err := json.Unmarshal([]byte(blob), &doc); err != nil {
		fmt.Fprintln(os.Stderr, "unmarshal any:", err)
		os.Exit(1)
	}

	fmt.Println("--- dynamic JSON ---")
	// GOTCHA: every JSON number becomes float64 — even "3".
	replicas := doc["replicas"].(float64)
	fmt.Printf("replicas: %v (Go type %T!)\n", int(replicas), doc["replicas"])

	// Nested objects are map[string]any; arrays are []any.
	labels := doc["labels"].(map[string]any)
	fmt.Println("team label:", labels["team"])
	for _, p := range doc["ports"].([]any) {
		fmt.Println("port:", int(p.(float64)))
	}

	// Use the comma-ok form when a key might be missing or mistyped —
	// a bare assertion on a missing key would panic.
	if v, ok := doc["healthy"].(bool); ok {
		fmt.Println("healthy:", v)
	}

	// -------------------- Streaming: Encoder/Decoder --------------------
	// For HTTP bodies and files, skip the []byte middleman:
	// json.NewDecoder(resp.Body).Decode(&v) / json.NewEncoder(w).Encode(v).
	fmt.Println("--- Encoder straight to stdout ---")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"streamed": true, "to": "stdout"})
}
