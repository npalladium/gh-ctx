package main

import "strings"

// shellQuote single-quotes s for safe use in POSIX shell, so tokens containing
// shell metacharacters survive `eval "$(gh ctx env)"`.
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// Close the quote, emit an escaped literal quote, reopen: ' -> '\''
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
