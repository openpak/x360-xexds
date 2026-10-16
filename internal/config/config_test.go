package config

import "testing"

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestDefaults(t *testing.T) {
	c, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":3074" || c.Key != "" || c.Debug {
		t.Fatalf("defaults: %+v", c)
	}
}

func TestOverrides(t *testing.T) {
	c, err := Load(env(map[string]string{"X360_XEXDS_ADDR": "127.0.0.1:4000", "X360_XEXDS_KEY": "0123456789abcdef", "X360_XEXDS_DEBUG": "true"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != "127.0.0.1:4000" || c.Key != "0123456789abcdef" || !c.Debug {
		t.Fatalf("overrides: %+v", c)
	}
}

func TestShortKeyRejected(t *testing.T) {
	if _, err := Load(env(map[string]string{"X360_XEXDS_KEY": "short"})); err == nil {
		t.Fatal("short key accepted")
	}
}
