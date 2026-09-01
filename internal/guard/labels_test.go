package guard

import "testing"

func TestValidateNilAllowsAll(t *testing.T) {
	g := New(nil)
	if err := g.Validate([]string{"cualquier-cosa"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRejectsUnknown(t *testing.T) {
	g := New([]string{"lead-caliente", "lead-tibio"})
	err := g.Validate([]string{"super-mega-lead"})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !contains(msg, "super-mega-lead") || !contains(msg, "lead-caliente") {
		t.Errorf("msg=%s", msg)
	}
}

func TestValidateAllowsListed(t *testing.T) {
	g := New([]string{"lead-caliente", "cotizacion-enviada"})
	if err := g.Validate([]string{"lead-caliente"}); err != nil {
		t.Fatal(err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
