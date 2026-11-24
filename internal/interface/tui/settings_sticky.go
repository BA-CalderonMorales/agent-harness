package tui

// pinnedSection names the section to pin above the settings list, or ""
// when nothing needs pinning. The top visible row is the first row not
// wholly above offset; its section is pinned only when that section's
// inline header line has scrolled above the window. A header at or below
// offset is already on screen, since it precedes a visible row.
//
// rowEnd[i] is the line after row i; headerLine[i] is the line of row
// i's section header, or -1 for a row without a category.
func pinnedSection(settings []Setting, rowEnd, headerLine []int, offset int) string {
	for i, end := range rowEnd {
		if end <= offset {
			continue
		}
		if headerLine[i] >= 0 && headerLine[i] < offset {
			return settings[i].Category
		}
		return ""
	}
	return ""
}
