package services

import "testing"

func TestOwnerAndFolder(t *testing.T) {
	cases := []struct {
		path   string
		id     uint
		folder string
		ok     bool
	}{
		{"12/a.txt", 12, "", true},
		{"12/docs/2024/05/a.txt", 12, "docs/2024/05", true},
		{"a.txt", 0, "", false},
		{"abc/a.txt", 0, "", false},
		{"012/a.txt", 0, "", false},
		{"0/a.txt", 0, "", false},
	}
	for _, c := range cases {
		id, folder, ok := ownerAndFolder(c.path)
		if id != c.id || folder != c.folder || ok != c.ok {
			t.Fatalf("%s: got %d %q %v", c.path, id, folder, ok)
		}
	}
}
