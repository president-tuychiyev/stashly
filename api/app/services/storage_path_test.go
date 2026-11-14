package services

import "testing"

func TestStoragePathFromURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://cdn.example.com/storage/12/a/b.jpg", "12/a/b.jpg"},
		{"http://localhost:8080/storage/12/a%20b.jpg?v=2", "12/a b.jpg"},
		{"/storage/12/a/b.jpg", "12/a/b.jpg"},
		{"12/a/b.jpg", "12/a/b.jpg"},
		{"https://x.test/prefix/storage/12/storage/b.jpg", "12/storage/b.jpg"},
		{"https://x.test/storage/", ""},
		{"https://x.test", ""},
		{"", ""},
		{"://bad", ""},
	}
	for _, c := range cases {
		if got := StoragePathFromURL(c.raw); got != c.want {
			t.Fatalf("%q: got %q want %q", c.raw, got, c.want)
		}
	}
}
