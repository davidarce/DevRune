package diff

import (
	"testing"
)

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		old  []byte
		new  []byte
		want []DiffLine
	}{
		{
			name: "identical texts produce only context lines",
			old:  []byte("a\nb\nc\n"),
			new:  []byte("a\nb\nc\n"),
			want: []DiffLine{
				{Kind: "context", Text: "a"},
				{Kind: "context", Text: "b"},
				{Kind: "context", Text: "c"},
			},
		},
		{
			name: "line added at end",
			old:  []byte("a\nb\n"),
			new:  []byte("a\nb\nc\n"),
			want: []DiffLine{
				{Kind: "context", Text: "a"},
				{Kind: "context", Text: "b"},
				{Kind: "added", Text: "c"},
			},
		},
		{
			name: "line removed",
			old:  []byte("a\nb\nc\n"),
			new:  []byte("a\nc\n"),
			want: []DiffLine{
				{Kind: "context", Text: "a"},
				{Kind: "removed", Text: "b"},
				{Kind: "context", Text: "c"},
			},
		},
		{
			name: "substitution: one line changes (removed + added)",
			old:  []byte("a\nb\n"),
			new:  []byte("a\nc\n"),
			want: []DiffLine{
				{Kind: "context", Text: "a"},
				{Kind: "removed", Text: "b"},
				{Kind: "added", Text: "c"},
			},
		},
		{
			name: "empty old produces only added lines",
			old:  []byte(""),
			new:  []byte("a\nb\n"),
			want: []DiffLine{
				{Kind: "added", Text: "a"},
				{Kind: "added", Text: "b"},
			},
		},
		{
			name: "empty new produces only removed lines",
			old:  []byte("a\nb\n"),
			new:  []byte(""),
			want: []DiffLine{
				{Kind: "removed", Text: "a"},
				{Kind: "removed", Text: "b"},
			},
		},
		{
			name: "both empty produces no lines",
			old:  []byte(""),
			new:  []byte(""),
			want: []DiffLine{},
		},
		{
			name: "plan checkpoint: Diff(a, a) returns only context",
			old:  []byte("hello\nworld\n"),
			new:  []byte("hello\nworld\n"),
			want: []DiffLine{
				{Kind: "context", Text: "hello"},
				{Kind: "context", Text: "world"},
			},
		},
		{
			name: "plan checkpoint: Diff(a\\nb\\n, a\\nc\\n) returns context a, removed b, added c",
			old:  []byte("a\nb\n"),
			new:  []byte("a\nc\n"),
			want: []DiffLine{
				{Kind: "context", Text: "a"},
				{Kind: "removed", Text: "b"},
				{Kind: "added", Text: "c"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Diff(tc.old, tc.new)

			if len(got) != len(tc.want) {
				t.Fatalf("Diff() returned %d lines, want %d\ngot:  %#v\nwant: %#v",
					len(got), len(tc.want), got, tc.want)
			}

			for i, line := range got {
				if line.Kind != tc.want[i].Kind || line.Text != tc.want[i].Text {
					t.Errorf("line[%d]: got {Kind:%q, Text:%q}, want {Kind:%q, Text:%q}",
						i, line.Kind, line.Text, tc.want[i].Kind, tc.want[i].Text)
				}
			}
		})
	}
}

func TestDiffAllContextKinds(t *testing.T) {
	result := Diff([]byte("x\n"), []byte("x\n"))
	for _, line := range result {
		if line.Kind != "context" {
			t.Errorf("identical input: expected all context, got Kind=%q", line.Kind)
		}
	}
}
