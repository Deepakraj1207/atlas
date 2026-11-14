package ai

import (
	"os"
	"os/user"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// MaxQuestionChars caps what one question sends.
const MaxQuestionChars = 2000

// maxLineChars caps one line, such as a long package description.
const maxLineChars = 300

// redactor removes things that identify the user from a question.
type redactor struct {
	home, user, host string
}

// currentRedactor describes this computer and its user.
func currentRedactor() redactor {
	var r redactor
	r.home, _ = os.UserHomeDir()
	if u, err := user.Current(); err == nil {
		name := u.Username
		if i := strings.LastIndexAny(name, `\/`); i >= 0 { // DOMAIN\name on Windows
			name = name[i+1:]
		}
		r.user = name
	}
	r.host, _ = os.Hostname()
	return r
}

// genericNames are account and computer names that identify no one.
var genericNames = []string{"root", "admin", "administrator", "user", "guest", "localhost", "runner"}

func identifying(s string) bool {
	return utf8.RuneCountInString(s) >= 3 && !slices.Contains(genericNames, strings.ToLower(s))
}

// apply replaces the home folder with ~, then the user name and computer
// name wherever they appear as whole words.
func (r redactor) apply(q string) string {
	if len(r.home) >= 3 {
		for _, h := range []string{r.home, strings.ReplaceAll(r.home, `\`, "/"), strings.ReplaceAll(r.home, "/", `\`)} {
			q = regexp.MustCompile(`(?i)`+regexp.QuoteMeta(h)).ReplaceAllString(q, "~")
		}
	}
	for _, w := range []struct{ name, with string }{{r.user, "[user]"}, {r.host, "[computer]"}} {
		if identifying(w.name) {
			q = regexp.MustCompile(`(?i)(^|[^\pL\pN_])`+regexp.QuoteMeta(w.name)+`($|[^\pL\pN_])`).ReplaceAllString(q, "${1}"+w.with+"${2}")
		}
	}
	return q
}

// shorten caps each line and the whole question.
func shorten(q string) string {
	lines := strings.Split(q, "\n")
	for i, l := range lines {
		if utf8.RuneCountInString(l) > maxLineChars {
			lines[i] = string([]rune(l)[:maxLineChars-1]) + "…"
		}
	}
	q = strings.Join(lines, "\n")
	if utf8.RuneCountInString(q) <= MaxQuestionChars {
		return q
	}
	cut := string([]rune(q)[:MaxQuestionChars-len("\n…")])
	if i := strings.LastIndex(cut, "\n"); i > 0 {
		cut = cut[:i]
	}
	return cut + "\n…"
}
