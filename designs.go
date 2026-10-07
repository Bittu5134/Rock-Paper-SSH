package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// designs.go — the three choice icons. Tiny terminal-native art: four rows
// tall, drawn with unambiguous single-width box-drawing characters, colored
// in each choice's vibrant palette color (see ui.go).

var choiceDesigns = [3][4]string{
	{" /-\\ ", "/###\\", "|###|", " \\_/ "}, // stone
	{"┌───┐", "│≡≡≡│", "│≡≡≡│", "└───┘"},    // paper
	{"╲   ╱", " ╲ ╱ ", "  X  ", " o o "},    // scissors
}

// renderChoices styles the designs with the session renderer and lays the
// three choices in a row. Every block gets the same width so the icon
// columns and labels stay aligned.
func renderChoices(r *lipgloss.Renderer, selected int) string {
	// widest icon row or name, plus a little breathing room
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
