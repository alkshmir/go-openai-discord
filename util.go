package main

import (
	"container/list"
	"strings"
	"unicode/utf8"
)

type Stack struct {
	v *list.List
}

func NewStack() *Stack {
	return &Stack{v: list.New()}
}

func (s *Stack) Push(v interface{}) {
	s.v.PushBack(v)
}

func (s *Stack) Pop() interface{} {
	b := s.v.Back()
	if b == nil {
		return nil
	}
	return s.v.Remove(b)
}

func (s *Stack) Len() int {
	return s.v.Len()
}

func splitMessage(input string, maxLen int) []string {
	lines := strings.Split(input, "\n")
	var results []string
	var currentMessage strings.Builder

	codeStack := NewStack()
	backupStack := NewStack()
	currentLength := 0

	for _, line := range lines {
		currentLength += utf8.RuneCountInString(line) + 1

		trimmedLine := strings.TrimRight(line, " ")
		if strings.HasPrefix(trimmedLine, "```") {
			if codeStack.Len() > 0 {
				// Pop from stack
				codeStack.Pop()
				currentLength -= 4
			} else {
				// Push to stack
				codeStack.Push(trimmedLine)
				currentLength += 4
			}
		}

		if currentLength+len(line)+1 > maxLen {
			if codeStack.Len() > 0 {
				// We are in a code block, add the code block closure
				for codeStack.Len() != 0 {
					currentMessage.WriteString("```\n")
					backupStack.Push(codeStack.Pop())
				}
				results = append(results, currentMessage.String())
				currentMessage.Reset()
				currentLength = 0

				for backupStack.Len() != 0 {
					v := backupStack.Pop()
					currentMessage.WriteString(v.(string) + "\n")
					currentLength += utf8.RuneCountInString(v.(string)) + 4
					codeStack.Push(v)
				}

			} else {
				results = append(results, currentMessage.String())
				currentMessage.Reset()
				currentLength = 0
			}
		}

		currentMessage.WriteString(line + "\n")
		currentLength += utf8.RuneCountInString(line) + 1
	}

	if currentMessage.Len() > 0 {

		results = append(results, currentMessage.String())
	}

	return results
}
