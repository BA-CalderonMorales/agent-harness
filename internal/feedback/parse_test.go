package feedback

import "testing"

func TestParseArgsReadsEachCategory(t *testing.T) {
	cases := []struct {
		args string
		want Category
		text string
	}{
		{"bug crash on save", CategoryBug, "crash on save"},
		{"annoying the spinner never stops", CategoryAnnoying, "the spinner never stops"},
		{"missing a way to pin sessions", CategoryMissing, "a way to pin sessions"},
		{"other anything else", CategoryOther, "anything else"},
	}
	for _, tc := range cases {
		gotCat, gotText := ParseArgs(tc.args)
		if gotCat != tc.want || gotText != tc.text {
			t.Errorf("ParseArgs(%q) = (%q, %q), want (%q, %q)", tc.args, gotCat, gotText, tc.want, tc.text)
		}
	}
}

func TestParseArgsWithoutCategoryIsOther(t *testing.T) {
	gotCat, gotText := ParseArgs("the whole argument is the text")
	if gotCat != CategoryOther || gotText != "the whole argument is the text" {
		t.Fatalf("ParseArgs without category = (%q, %q), want (other, whole argument)", gotCat, gotText)
	}
}

func TestParseArgsEmptyTextIsReportedAsEmpty(t *testing.T) {
	for _, args := range []string{"", "   ", "bug", "  annoying  ", "other"} {
		_, text := ParseArgs(args)
		if text != "" {
			t.Errorf("ParseArgs(%q) text = %q, want empty", args, text)
		}
	}
}

func TestParseArgsTrimsSurroundingSpace(t *testing.T) {
	cat, text := ParseArgs("   missing    a dark mode   ")
	if cat != CategoryMissing || text != "a dark mode" {
		t.Fatalf("ParseArgs = (%q, %q), want (missing, a dark mode)", cat, text)
	}
}
