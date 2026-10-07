package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var choiceDesigns = [3][4]string{
	{" /-\\ ", "/###\\", "|###|", " \\_/ "},
	{"┌───┐", "│≡≡≡│", "│≡≡≡│", "└───┘"},
	{"╲   ╱", " ╲ ╱ ", "  X  ", " o o "},
}

func renderChoicesCompact(r *lipgloss.Renderer, selected int) string {
	blocks := make([]string, len(names))
	for i, name := range names {
		col := choiceColor(i)
		st := r.NewStyle().Foreground(col).Padding(0, 1)
		if i == selected {
			st = st.Bold(true).Border(lipgloss.RoundedBorder()).BorderForeground(col)
		} else {
			st = st.Border(lipgloss.HiddenBorder())
		}
		blocks[i] = st.Render(name)
	}
	return lipgloss.JoinHorizontal(lipgloss.Center, blocks...)
}

func renderChoices(r *lipgloss.Renderer, selected int) string {
	inner := 0
	for i, design := range choiceDesigns {
		for _, row := range design {
			inner = max(inner, lipgloss.Width(row))
		}
		inner = max(inner, lipgloss.Width(names[i]))
	}
	inner += 2

	blocks := make([]string, len(names))
	for i, design := range choiceDesigns {
		col := choiceColor(i)
		icon := r.NewStyle().Foreground(col).Render(strings.Join(design[:], "\n"))

		labelStyle := r.NewStyle().Foreground(col)
		if i == selected {
			labelStyle = labelStyle.Bold(true).Underline(true)
		}
		block := lipgloss.JoinVertical(lipgloss.Center, icon, labelStyle.Render(names[i]))
		blocks[i] = r.NewStyle().Width(inner).Align(lipgloss.Center).Padding(0, 1).Render(block)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
}
