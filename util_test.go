package main

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   []string
	}{
		{
			name:   "EmptyInput",
			input:  "",
			maxLen: 20,
			want:   nil,
		},
		{
			name:   "SingleLineNoTrailingNewline",
			input:  "Hello, world!",
			maxLen: 20,
			want:   []string{"Hello, world!"},
		},
		{
			name:   "MultiLineWithinLimit",
			input:  "12345\n67890",
			maxLen: 20,
			want:   []string{"12345\n67890"},
		},
		{
			name:   "MultiByteWithinLimit",
			input:  "あいうえお\nかきくけこ",
			maxLen: 20,
			want:   []string{"あいうえお\nかきくけこ"},
		},
		{
			name:   "MultiLineSplitAtNewline",
			input:  "aaaaa\nbbbbb\nccccc",
			maxLen: 12,
			want:   []string{"aaaaa\nbbbbb", "ccccc"},
		},
		{
			name:   "SingleLineExceedsMaxLen",
			input:  strings.Repeat("あ", 25),
			maxLen: 10,
			want: []string{
				strings.Repeat("あ", 10),
				strings.Repeat("あ", 10),
				strings.Repeat("あ", 5),
			},
		},
		{
			name: "CodeBlockSplit",
			input: strings.Join([]string{
				"```go",
				"line1",
				"line2",
				"line3",
				"```",
			}, "\n"),
			maxLen: 22,
			want: []string{
				"```go\nline1\nline2\n```",
				"```go\nline3\n```",
			},
		},
		{
			name: "CodeBlockLongSingleLine",
			input: strings.Join([]string{
				"```go",
				strings.Repeat("a", 15),
				"```",
			}, "\n"),
			maxLen: 20,
			want: []string{
				"```go\n" + strings.Repeat("a", 10) + "\n```",
				"```go\n" + strings.Repeat("a", 5) + "\n```",
			},
		},
		{
			name: "TextBeforeCodeBlockFlushesBeforeOpeningFence",
			input: strings.Join([]string{
				"hello",
				"```go",
				"code1",
				"```",
			}, "\n"),
			maxLen: 15,
			want: []string{
				"hello",
				"```go\ncode1\n```",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitMessage(tt.input, tt.maxLen)

			for i, chunk := range got {
				if n := utf8.RuneCountInString(chunk); n > tt.maxLen {
					t.Errorf("chunk[%d] length = %d runes, exceeds maxLen %d: %q", i, n, tt.maxLen, chunk)
				}
				if chunk == "" {
					t.Errorf("chunk[%d] is empty", i)
				}
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitMessage(%q, %d)\ngot:  %#v\nwant: %#v", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
