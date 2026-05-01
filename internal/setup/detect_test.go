package setup

import "testing"

func TestParseObsidianRegistry_singleVault(t *testing.T) {
	data := []byte(`{"vaults":{"4cb8a23875af1fa9":{"path":"/Users/yaoyi/hoard","ts":1775749049442,"open":true}}}`)
	got := parseObsidianRegistry(data)
	if len(got) != 1 {
		t.Fatalf("len: got %d, want 1", len(got))
	}
	if got[0].Path != "/Users/yaoyi/hoard" {
		t.Errorf("Path: got %q", got[0].Path)
	}
	if got[0].Name != "hoard" {
		t.Errorf("Name: got %q want %q", got[0].Name, "hoard")
	}
}

func TestParseObsidianRegistry_multipleVaultsSortedByName(t *testing.T) {
	data := []byte(`{"vaults":{
		"a":{"path":"/x/zebra"},
		"b":{"path":"/y/alpha"},
		"c":{"path":"/z/mango"}
	}}`)
	got := parseObsidianRegistry(data)
	if len(got) != 3 {
		t.Fatalf("len: got %d, want 3", len(got))
	}
	want := []string{"alpha", "mango", "zebra"}
	for i, v := range got {
		if v.Name != want[i] {
			t.Errorf("[%d] Name: got %q want %q", i, v.Name, want[i])
		}
	}
}

func TestParseObsidianRegistry_skipsEmptyPath(t *testing.T) {
	data := []byte(`{"vaults":{"a":{"path":""},"b":{"path":"/ok"}}}`)
	got := parseObsidianRegistry(data)
	if len(got) != 1 || got[0].Path != "/ok" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseObsidianRegistry_invalidJSON(t *testing.T) {
	if got := parseObsidianRegistry([]byte("not json")); got != nil {
		t.Errorf("expected nil for invalid JSON, got %+v", got)
	}
}

func TestParseObsidianRegistry_emptyVaults(t *testing.T) {
	data := []byte(`{"vaults":{}}`)
	got := parseObsidianRegistry(data)
	if len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}
