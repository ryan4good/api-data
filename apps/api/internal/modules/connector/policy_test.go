package connector

import (
	"reflect"
	"testing"
)

func TestNarrowHostAllowlistCannotWidenGlobalPolicy(t *testing.T) {
	global := []string{"api.test", "shared.test"}
	if got := NarrowHostAllowlist(global, []string{"api.test", "evil.test"}); !reflect.DeepEqual(got, []string{"api.test"}) {
		t.Fatalf("got=%#v", got)
	}
	if got := NarrowHostAllowlist(nil, []string{"evil.test"}); len(got) != 0 {
		t.Fatalf("empty global was widened: %#v", got)
	}
	if got := NarrowHostAllowlist(global, nil); !reflect.DeepEqual(got, global) {
		t.Fatalf("inherit got=%#v", got)
	}
}
