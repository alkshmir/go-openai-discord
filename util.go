package main

import (
	"strings"
	"unicode/utf8"
)

const closingFence = "\n```"

func splitMessage(input string, maxLen int) []string {
	if input == "" || maxLen <= 0 {
		return nil
	}

	const closingFenceLen = 4 // len("\n```")

	lines := strings.Split(input, "\n")
	var results []string
	var buf strings.Builder
	curLen := 0
	openFence := ""
	hasContentSinceFence := false

	appendLine := func(s string, runeLen int) {
		if curLen > 0 {
			buf.WriteByte('\n')
			curLen++
		}
		buf.WriteString(s)
		curLen += runeLen
	}

	flush := func() {
		if curLen == 0 {
			return
		}
		if openFence != "" {
			buf.WriteString(closingFence)
		}
		results = append(results, buf.String())
		buf.Reset()
		curLen = 0
		if openFence != "" {
			buf.WriteString(openFence)
			curLen = utf8.RuneCountInString(openFence)
			hasContentSinceFence = false
		}
	}

	for _, line := range lines {
		trimmedRight := strings.TrimRight(line, " ")
		isFence := strings.HasPrefix(trimmedRight, "```")
		lineLen := utf8.RuneCountInString(line)

		// Closing an open code block.
		if isFence && openFence != "" {
			sep := 0
			if curLen > 0 {
				sep = 1
			}
			if curLen > 0 && curLen+sep+lineLen > maxLen {
				flush()
			}
			appendLine(line, lineLen)
			openFence = ""
			hasContentSinceFence = false
			continue
		}

		// Opening a new code block.
		if isFence && openFence == "" {
			// If there is existing content and not enough room for this fence,
			// at least 1 character of content, and the closing fence, flush first.
			if curLen > 0 && curLen+1+lineLen+1+1+closingFenceLen > maxLen {
				flush()
			}
			appendLine(line, lineLen)
			openFence = trimmedRight
			hasContentSinceFence = false
			continue
		}

		// Regular line (either inside or outside a code block).
		reserve := 0
		if openFence != "" {
			reserve = closingFenceLen
		}

		sep := 0
		if curLen > 0 {
			sep = 1
		}

		// If the line does not fit in the current chunk, flush if there is already content to flush.
		if curLen+sep+lineLen+reserve > maxLen && curLen > 0 && (openFence == "" || hasContentSinceFence) {
			flush()
			sep = 0
			if curLen > 0 {
				sep = 1
			}
		}

		if curLen+sep+lineLen+reserve <= maxLen {
			appendLine(line, lineLen)
			if openFence != "" {
				hasContentSinceFence = true
			}
			continue
		}

		// Single line itself exceeds the remaining capacity of a chunk; split by runes.
		runes := []rune(line)
		for len(runes) > 0 {
			sep = 0
			if curLen > 0 {
				sep = 1
			}
			avail := maxLen - curLen - sep - reserve
			if avail < 1 {
				avail = 1
			}
			if avail > len(runes) {
				avail = len(runes)
			}
			part := string(runes[:avail])
			appendLine(part, avail)
			if openFence != "" {
				hasContentSinceFence = true
			}
			runes = runes[avail:]
			if len(runes) > 0 {
				flush()
			}
		}
	}

	if curLen > 0 {
		results = append(results, buf.String())
	}

	return results
}
