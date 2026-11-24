package feedback

import "strings"

// ParseArgs splits the argument of /feedback into a category and the text.
// A first word that names a category selects it and the rest is the text;
// any other input is CategoryOther and the whole argument is the text.
// The returned text is empty when nothing was written after the category,
// which the caller reports as a usage error rather than saving.
func ParseArgs(args string) (Category, string) {
	args = strings.TrimSpace(args)
	word, rest, _ := strings.Cut(args, " ")
	switch c := Category(word); c {
	case CategoryBug, CategoryAnnoying, CategoryMissing, CategoryOther:
		return c, strings.TrimSpace(rest)
	}
	return CategoryOther, args
}
