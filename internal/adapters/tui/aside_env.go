package tui

import (
	"strings"

	"charm.land/lipgloss/v2"

	"fuku/internal/model"
)

// asideEnvTab renders the env tab with values hard-wrapped to fill the row
func (m Model) asideEnvTab(service *model.Service, innerWidth int) string {
	merged := m.dotenvEntries(service)
	if len(merged) == 0 {
		return m.theme.PlaceholderStyle.Render("no environment variables available")
	}

	rowAvailable := asideContentWidth(innerWidth)
	if rowAvailable < 1 {
		return ""
	}

	rows := make([]cardRow, 0, len(merged))
	for _, kv := range merged {
		rows = append(rows, cardRow{label: kv.Key, value: kv.Value})
	}

	labelWidth := computeLabelWidth(rows)
	separator := asideRowIndent + m.theme.PanelMutedStyle.Render(strings.Repeat(asideDottedChar, rowAvailable))

	lines := make([]string, 0, len(rows)*3+1)
	lines = append(lines, asideTitleIndent+m.theme.AsideSectionTitleStyle.Render("environment"))

	for i, row := range rows {
		if i > 0 {
			lines = append(lines, separator)
		}

		lines = append(lines, m.envWrappedRow(row, labelWidth, rowAvailable)...)
	}

	return strings.Join(lines, "\n")
}

// envWrappedRow renders one env entry with its value hard-wrapped across continuation lines
func (m Model) envWrappedRow(row cardRow, labelWidth, available int) []string {
	labelStyled := m.theme.PanelMutedStyle.Render(row.label)
	labelDisplayWidth := lipgloss.Width(labelStyled)
	pad := max(labelWidth-labelDisplayWidth, 1)
	labelBlock := labelStyled + strings.Repeat(" ", pad)

	firstAvailable := available - lipgloss.Width(labelBlock)
	if firstAvailable < 1 {
		return []string{asideRowIndent + labelStyled}
	}

	chunks := wrapRunes(row.value, firstAvailable, available)
	if len(chunks) == 0 {
		return []string{asideRowIndent + labelBlock}
	}

	out := make([]string, 0, len(chunks))
	out = append(out, asideRowIndent+labelBlock+chunks[0])

	for _, c := range chunks[1:] {
		out = append(out, asideRowIndent+c)
	}

	return out
}

// dotenvEntries returns the merged .env entries for the service (nil when unavailable)
func (m Model) dotenvEntries(service *model.Service) []model.Env {
	return m.environment.Env(service.ID)
}

// wrapRunes splits s into rune chunks of firstWidth then width
func wrapRunes(s string, firstWidth, width int) []string {
	if s == "" || firstWidth <= 0 || width <= 0 {
		return nil
	}

	runes := []rune(s)

	var out []string

	chunk := firstWidth
	for len(runes) > 0 {
		end := min(chunk, len(runes))
		out = append(out, string(runes[:end]))
		runes = runes[end:]
		chunk = width
	}

	return out
}
