package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tiny terminal-native designs: four rows tall, no image files or protocols.
// The ASCII/box-drawing shapes stay recognizable in color and monochrome.
var choiceDesigns = [3][4]string{
	{" /-\\ ", "/###\\", "|###|", " \\_/ "},
	{"┌───┐", "│≡≡≡│", "│≡≡≡│", "└───┘"},
	{"╲   ╱", " ╲ ╱ ", "  X  ", " o o "},
}

var choiceColors = [3]lipgloss.TerminalColor{
	lipgloss.Color("#A8A29E"), // stone gray
	lipgloss.Color("#FDE68A"), // paper cream
	lipgloss.Color("#7DD3FC"), // shears steel blue
}

// renderChoices styles compact designs using the connected client's color
// profile, highlights the selected option, and lays the three choices in a row.
func renderChoices(r *lipgloss.Renderer, selected int) string {
	blocks := make([]string, len(names))
	for i, design := range choiceDesigns {
		icon := r.NewStyle().Foreground(choiceColors[i]).Render(strings.Join(design[:], "\n"))
		labelStyle := r.NewStyle().Foreground(choiceColors[i]).Bold(i == selected)
		if i == selected {
			labelStyle = labelStyle.Underline(true)
		}
		label := labelStyle.Render(names[i])
		block := lipgloss.JoinVertical(lipgloss.Center, icon, label)
		blocks[i] = lipgloss.NewStyle().Width(9).Align(lipgloss.Center).Padding(0, 1).Render(block)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, blocks...)
}
