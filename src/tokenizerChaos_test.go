package main

import (
	"bytes"
	"testing"
)

type ChaosTests struct {
	name              string
	input             string
	expectedTokenType TokenType
}

func makeZeroTerminatedString(str string) string {
	return str + "\000"
}

func TestTokenTypeCount(t *testing.T) {
	currentTotal := 10
	if int(tokCount) != currentTotal {
		t.Errorf("Expected tokCount to be '%d' but found '%d'", currentTotal, tokCount)
	}
}

func TestTokenizerChaosSingleToken(t *testing.T) {
	tests := []ChaosTests{
		{"[Option1]::TokenizeNumbers", "12345", tokNumber},
		{"[Option1]::TokenizeVarType", "Bool", tokVarType},
		{"[Option1]::TokenizerIdentifier", "something", tokIdentifier},
		{"[Option2]::TokenizerIdentifier", "something1", tokIdentifier},
		{"[Option1]::TokenizeKeyword", "let", tokLet},
		{"[Option1]::TokenizeAssignment", "=", tokAssignment},
		{"[Option1]::TokenizePlus", "+", tokPlus},
		{"[Option1]::SkipComment", "// something that is not correct\n", tokNewline},
		{"[Option2]::SkipComment", "/** something that is not correct**/\n", tokNewline},
		{"[Option3]::SkipComment", "/** /** something that is not correct **/**/\n", tokNewline},
		{"[Option1]::SkipEmpty", "                 ", tokEndOfFile},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inputValue := bytes.Buffer{}
			inputValue.WriteString(makeZeroTerminatedString(tt.input))
			ts := tokenizeChaos("/some/file/name", &inputValue)
			if val := ts.data[0]; val.tokType != tt.expectedTokenType {
				t.Errorf(
					"Expected token type to be '%s', but found '%s'",
					tt.expectedTokenType.asString(),
					val.tokType.asString(),
				)
			}
		})
	}
}
