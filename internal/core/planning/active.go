package planning

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var dateLinePattern = regexp.MustCompile("(?m)^-\\s*Date:\\s*`([0-9]{4}-[0-9]{2}-[0-9]{2})`\\s*$")

type activeDomain struct {
	Date       string
	GoalPath   string
	PlanPath   string
	NextAction string
}

func loadActive(root, domain string, now time.Time) (activeDomain, error) {
	base := filepath.Join(root, "plans", domain)
	indexPath := filepath.Join(base, "PLAN.md")

	index, err := os.ReadFile(indexPath)
	if err != nil {
		return activeDomain{}, fmt.Errorf("read plan index: %w", err)
	}

	date := activeDateFromIndex(string(index))
	if date == "" {
		date, err = latestDatedDomain(base)
		if err != nil {
			return activeDomain{}, err
		}
	}
	if date == "" {
		date = now.Format(dateLayout)
	}

	goalPath := filepath.Join(base, date, "GOAL.md")
	planPath := filepath.Join(base, date, "PLAN.md")

	goalBytes, err := os.ReadFile(goalPath)
	if err != nil {
		return activeDomain{}, fmt.Errorf("read active goal: %w", err)
	}
	planBytes, err := os.ReadFile(planPath)
	if err != nil {
		return activeDomain{}, fmt.Errorf("read active plan: %w", err)
	}

	next := nextAction(string(planBytes))
	if next == "" {
		next = nextAction(string(goalBytes))
	}
	if next == "" {
		return activeDomain{}, errors.New("active goal/plan has no actionable Today or unchecked item")
	}

	return activeDomain{
		Date:       date,
		GoalPath:   goalPath,
		PlanPath:   planPath,
		NextAction: next,
	}, nil
}

func activeDateFromIndex(index string) string {
	match := dateLinePattern.FindStringSubmatch(index)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func latestDatedDomain(base string) (string, error) {
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", fmt.Errorf("read plan domain: %w", err)
	}

	var dates []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, err := time.Parse(dateLayout, name); err == nil {
			dates = append(dates, name)
		}
	}
	sort.Strings(dates)
	if len(dates) == 0 {
		return "", nil
	}
	return dates[len(dates)-1], nil
}

func nextAction(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ] ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "- [ ] "))
		}
	}

	today := false
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if isTodayHeading(trimmed) {
			today = true
			continue
		}
		if today && strings.HasPrefix(trimmed, "## ") {
			return ""
		}
		if !today || trimmed == "" {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")
		trimmed = strings.TrimPrefix(trimmed, "[ ] ")
		trimmed = strings.TrimSpace(trimmed)
		if trimmed != "" && !strings.HasPrefix(trimmed, "[x] ") {
			return trimmed
		}
	}
	return ""
}

func isTodayHeading(line string) bool {
	return line == "Today" || line == "## Today"
}
