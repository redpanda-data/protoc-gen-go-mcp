package gen

import (
	"testing"
)

func FuzzMangleHeadIfTooLong(f *testing.F) {
	f.Add("short", 64)
	f.Add("some.very.long.fully.qualified.protobuf.method.name.that.exceeds.the.limit", 64)
	f.Add("", 0)
	f.Add("", -1)
	f.Add("a", 1)
	f.Add("abcdef", 6)
	f.Add("abcdefg", 6)
	f.Add("x", 0)
	f.Add("hello_world_test", 10)
	f.Add("a.b.c.d.e.f.g.h.i.j.k.l.m", 8)

	f.Fuzz(func(t *testing.T, name string, maxLen int) {
		// Must never panic
		result := MangleHeadIfTooLong(name, maxLen)

		// If maxLen > 0, result length must not exceed maxLen
		if maxLen > 0 && len(result) > maxLen {
			t.Errorf("MangleHeadIfTooLong(%q, %d) returned %q with length %d > maxLen",
				name, maxLen, result, len(result))
		}
	})
}

func FuzzCleanComment(f *testing.F) {
	f.Add("")
	f.Add("A simple comment.")
	f.Add("buf:lint:FIELD_LOWER_SNAKE_CASE\nActual comment here.")
	f.Add("@ignore-comment something\nKeep this line.")
	f.Add("buf:lint:FOO\nbuf:lint:BAR\n@ignore-comment baz\nReal comment.")
	f.Add("no special prefixes at all")
	f.Add("\n\n\n")
	f.Add("buf:lint:")
	f.Add("@ignore-comment")
	f.Add("  buf:lint:INDENTED")

	f.Fuzz(func(t *testing.T, comment string) {
		// Must never panic
		_ = CleanComment(comment)
	})
}
