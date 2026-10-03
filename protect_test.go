package apirouter

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"
)

type protBase struct {
	ID     int    `json:"id"`
	Secret string `json:"base_secret,protect"`
}

type ProtEmbedded struct {
	Token string `json:"token,protect"`
	Kind  string `json:"kind"`
}

type protUser struct {
	protBase
	*ProtEmbedded
	Name     string            `json:"name"`
	Password string            `json:"password,protect"`
	Nested   *protUser         `json:"nested,omitempty"`
	Map      map[string]any    `json:"map,omitempty"`
	List     []protBase        `json:"list,omitempty"`
	Raw      jsontext.Value    `json:"raw,omitempty"`
	Tags     map[string]string `json:"tags"`
}

func TestProtectedFields(t *testing.T) {
	u := &protUser{
		protBase:     protBase{ID: 1, Secret: "s1"},
		ProtEmbedded: &ProtEmbedded{Token: "tok", Kind: "k"},
		Name:         "alice",
		Password:     "hunter2",
		Nested:       &protUser{Name: "bob", Password: "pw2"},
		Map:          map[string]any{"x": protBase{ID: 2, Secret: "s2"}},
		List:         []protBase{{ID: 3, Secret: "s3"}},
		Raw:          jsontext.Value(`{"password":"raw"}`),
	}

	full, err := json.Marshal(u, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}
	pub, err := json.Marshal(u, publicJsonOpts, json.Deterministic(true))
	if err != nil {
		t.Fatal(err)
	}

	expFull := `{"id":1,"base_secret":"s1","token":"tok","kind":"k","name":"alice","password":"hunter2","nested":{"id":0,"base_secret":"","name":"bob","password":"pw2","tags":{}},"map":{"x":{"id":2,"base_secret":"s2"}},"list":[{"id":3,"base_secret":"s3"}],"raw":{"password":"raw"},"tags":{}}`
	expPub := `{"id":1,"kind":"k","name":"alice","nested":{"id":0,"kind":"","name":"bob","tags":{}},"map":{"x":{"id":2}},"list":[{"id":3}],"raw":{"password":"raw"},"tags":{}}`
	if string(full) != expFull {
		t.Errorf("full:\n got %s\nwant %s", full, expFull)
	}
	if string(pub) != expPub {
		t.Errorf("public:\n got %s\nwant %s", pub, expPub)
	}
}

func TestProtectedFieldsPretty(t *testing.T) {
	buf := &bytes.Buffer{}
	err := json.MarshalEncode(jsontext.NewEncoder(buf, publicJsonOpts, jsontext.WithIndent("    ")), protBase{ID: 1, Secret: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if exp := "{\n    \"id\": 1\n}\n"; buf.String() != exp {
		t.Errorf("got %q want %q", buf.String(), exp)
	}
}
