package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	colTitle  = "#FF2FD6"
	colAccent = "#22F2FF"
	colWarn   = "#F9E71C"
	colBad    = "#FF4D6D"
	colGood   = "#3DF273"
	colDim    = "#737373"
)

const (
	colStone    = "#FFA657"
	colPaper    = colWarn
	colScissors = colBad
)

const (
	minTermW = 45
	minTermH = 16

	frameMargin    = 4
	maxContentW    = 100
	minContentW    = 42
	sideBySideMinW = 76
	leaderboardInW = 28
	panelGap       = 2
	minShareBarW   = 18
	maxShareBarW   = 70
)

type uiStyles struct {
	title     lipgloss.Style
	subtitle  lipgloss.Style
	timer     lipgloss.Style
	panel     lipgloss.Style
	panelHead lipgloss.Style
	help      lipgloss.Style
	label     lipgloss.Style
	win       lipgloss.Style
	lose      lipgloss.Style
	you       lipgloss.Style
	danger    lipgloss.Style
}

func newStyles(r *lipgloss.Renderer) *uiStyles {
	return &uiStyles{
		title:     r.NewStyle().Bold(true).Foreground(lipgloss.Color(colTitle)),
		subtitle:  r.NewStyle().Foreground(lipgloss.Color(colAccent)),
		timer:     r.NewStyle().Bold(true).Foreground(lipgloss.Color(colAccent)),
		panel:     r.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(colDim)).Padding(0, 1),
		panelHead: r.NewStyle().Bold(true).Foreground(lipgloss.Color(colAccent)),
		help:      r.NewStyle().Foreground(lipgloss.Color(colDim)),
		label:     r.NewStyle().Foreground(lipgloss.Color(colDim)),
		win:       r.NewStyle().Foreground(lipgloss.Color(colGood)),
		lose:      r.NewStyle().Foreground(lipgloss.Color(colBad)),
		you:       r.NewStyle().Foreground(lipgloss.Color(colTitle)).Bold(true),
		danger:    r.NewStyle().Foreground(lipgloss.Color(colBad)),
	}
}

func choiceColor(i int) lipgloss.TerminalColor {
	switch i {
	case 0:
		return lipgloss.Color(colStone)
	case 1:
		return lipgloss.Color(colPaper)
	default:
		return lipgloss.Color(colScissors)
	}
}

func renderTimer(r *lipgloss.Renderer, st *uiStyles, contentW int) string {
	left := int(timeLeft().Seconds()) + 1
	if left < 0 {
		left = 0
	}
	total := int(roundDuration.Seconds())
	if total <= 0 {
		total = 1
	}

	barLen := contentW / 2
	barLen = min(max(barLen, 10), 30)
	filled := left * barLen / total
	if filled > barLen {
		filled = barLen
	}

	frac := float64(left) / float64(total)
	col := colAccent
	switch {
	case frac <= 0.25:
		col = colBad
	case frac <= 0.5:
		col = colWarn
	}

	bar := r.NewStyle().Foreground(lipgloss.Color(col)).Render(strings.Repeat("▰", filled)) +
		r.NewStyle().Foreground(lipgloss.Color(colDim)).Render(strings.Repeat("▱", barLen-filled))

	return fmt.Sprintf("%s  %s", bar, st.timer.Foreground(lipgloss.Color(col)).Render(fmt.Sprintf("%ds", left)))
}

func renderShareBar(r *lipgloss.Renderer, st *uiStyles, dist [3]float64, total int, width int) string {
	head := st.panelHead.Render("PICKS ACROSS THE BOARD") + " " + st.help.Render("(tab to cycle)")
	if total == 0 || width < 10 {
		return lipgloss.JoinVertical(lipgloss.Left, head, st.label.Render("no picks yet — waiting for players"))
	}

	w0 := int(dist[0] / 100 * float64(width))
	w1 := int(dist[1] / 100 * float64(width))
	w2 := width - w0 - w1

	bar := r.NewStyle().Foreground(choiceColor(0)).Render(strings.Repeat("█", w0))
	bar += r.NewStyle().Foreground(choiceColor(1)).Render(strings.Repeat("█", w1))
	bar += r.NewStyle().Foreground(choiceColor(2)).Render(strings.Repeat("█", w2))

	legend := fmt.Sprintf("%s %s%%  %s %s%%  %s %s%%",
		r.NewStyle().Foreground(choiceColor(0)).Render("Stone"), fmtPct(dist[0]),
		r.NewStyle().Foreground(choiceColor(1)).Render("Paper"), fmtPct(dist[1]),
		r.NewStyle().Foreground(choiceColor(2)).Render("Scissors"), fmtPct(dist[2]))

	return lipgloss.JoinVertical(lipgloss.Left, head, bar, legend)
}

func fmtPct(p float64) string {
	return fmt.Sprintf("%5.1f", p)
}

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 2 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

func renderLeaderboard(st *uiStyles, board leaderboard, meSessionID string, maxContentLines int) string {
	head := st.panelHead.Render(fmt.Sprintf("LEADERBOARD (%d)", len(board)))
	if len(board) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, head, st.label.Render("nobody has points yet"))
	}

	availRows := max(1, maxContentLines-1)
	showCount := len(board)
	hidden := 0
	if showCount > availRows {
		if availRows > 1 {
			showCount = availRows - 1
			hidden = len(board) - showCount
		} else {
			showCount = 1
			hidden = len(board) - 1
		}
	}

	rows := make([]string, 0, showCount+2)
	rows = append(rows, head)
	for rank := 0; rank < showCount; rank++ {
		e := board[rank]
		nameText := truncate(e.user, 10)
		if e.sessionID == meSessionID {
			nameText = st.you.Render(truncate(e.user+" (you)", 11))
		}
		style := st.label
		if e.points > 0 || rank == 0 {
			style = st.win
		}
		rows = append(rows, fmt.Sprintf("%2d. %s %s", rank+1, nameText, style.Render(fmt.Sprintf("%d pts", e.points))))
	}
	if hidden > 0 {
		rows = append(rows, st.label.Render(fmt.Sprintf("    +%d more players", hidden)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func renderRoundSummary(r *lipgloss.Renderer, st *uiStyles, msg *roundEndMsg, meSessionID string, maxContentLines int) string {
	head := st.panelHead.Render("LAST ROUND")
	if msg == nil {
		return lipgloss.JoinVertical(lipgloss.Left, head, st.label.Render("waiting for first round..."))
	}
	serverMove := r.NewStyle().Bold(true).Foreground(choiceColor(msg.serverChoice)).Render(names[msg.serverChoice])
	lines := []string{head, fmt.Sprintf("%s  %s", st.label.Render("SERVER"), serverMove)}

	results := msg.sortedResults()
	availForResults := max(1, maxContentLines-2)

	showCount := len(results)
	hidden := 0
	if showCount > availForResults {
		if availForResults > 1 {
			showCount = availForResults - 1
			hidden = len(results) - showCount
		} else {
			showCount = 1
			hidden = len(results) - 1
		}
	}

	for i := 0; i < showCount; i++ {
		p := results[i]
		mark, markStyle := "·", st.label
		nameText := truncate(p.user, 6)
		if p.sessionID == meSessionID {
			nameText = st.you.Render(truncate(p.user+" (you)", 7))
		}
		switch p.outcome {
		case "win":
			mark, markStyle = "▲ WIN", st.win
		case "lose":
			mark, markStyle = "▼ LOSE", st.lose
		}
		delta := ""
		if p.delta > 0 {
			delta = st.win.Render(fmt.Sprintf("+%d", p.delta))
		} else if p.delta < 0 {
			delta = st.lose.Render(fmt.Sprintf("%d", p.delta))
		}
		lines = append(lines, fmt.Sprintf(" %s %s %s", nameText, markStyle.Render(mark), delta))
	}
	if hidden > 0 {
		lines = append(lines, st.label.Render(fmt.Sprintf("    +%d more players", hidden)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func renderTooSmall(r *lipgloss.Renderer, st *uiStyles, w, h int) string {
	title := r.NewStyle().Bold(true).Foreground(lipgloss.Color(colBad)).Render("⚠️  TERMINAL TOO SMALL")
	msg := st.label.Render("Please resize your terminal window")
	cur := fmt.Sprintf("Current size: %s", st.lose.Render(fmt.Sprintf("%d × %d", w, h)))
	req := fmt.Sprintf("Minimum size: %s", st.win.Render(fmt.Sprintf("%d × %d", minTermW, minTermH)))

	content := lipgloss.JoinVertical(lipgloss.Center, title, "", msg, "", cur, req)
	box := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colBad)).
		Padding(1, 3).
		Render(content)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
}

func renderFullPage(m model) string {
	r, st := m.renderer, m.styles

	w, h := m.width, m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	if w < minTermW || h < minTermH {
		return renderTooSmall(r, st, w, h)
	}

	contentW := min(max(w-frameMargin, minContentW), maxContentW)
	page := assemblePage(r, st, m, contentW, h)

	frame := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colAccent)).
		Padding(0, 1)

	rendered := frame.Render(page)
	if lipgloss.Height(rendered) > h || lipgloss.Width(rendered) > w {
		return renderTooSmall(r, st, w, h)
	}

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, rendered)
}

func assemblePage(r *lipgloss.Renderer, st *uiStyles, m model, contentW int, termH int) string {
	parts := []string{st.title.Render("ROCK  ·  PAPER  ·  SCISSORS")}
	if termH >= 24 {
		parts = append(parts, st.subtitle.Render("SSH battle — beat the board, take their points"))
	}
	parts = append(parts, st.help.Render("HINT: "+m.hint))
	header := lipgloss.JoinVertical(lipgloss.Center, parts...)

	timer := renderTimer(r, st, contentW)
	var choices string
	if termH < 22 {
		choices = renderChoicesCompact(r, m.choice)
	} else {
		choices = renderChoices(r, m.choice)
	}

	sideBySide := contentW >= sideBySideMinW

	headerSection := []string{header}
	if termH >= 22 {
		headerSection = append(headerSection, "")
	}
	headerSection = append(headerSection, timer)
	topBlock := lipgloss.JoinVertical(lipgloss.Center, headerSection...)

	headerLines := lipgloss.Height(topBlock)
	availForPanels := termH - 2 - headerLines // subtract outer frame (2) and header section

	if sideBySide {
		boardPanelW := leaderboardInW + 4 // rendered width of board panel (inner + border/padding)
		mainPanelW := contentW - boardPanelW - panelGap
		mainInnerW := max(10, mainPanelW-4)
		barW := min(max(mainInnerW-2, minShareBarW), maxShareBarW)

		mainContent := lipgloss.JoinVertical(lipgloss.Left, choices, renderShareBar(r, st, m.dist, m.total, barW))
		minMainH := lipgloss.Height(mainContent)

		// Remaining space after body panel border (2) and summary minimum panel (5 content + 2 border = 7)
		maxBodyPanelH := max(minMainH, availForPanels-7-2)

		// Determine leaderboard content lines allowed
		boardContent := renderLeaderboard(st, m.leaderboard, m.sessionID, maxBodyPanelH)
		boardH := lipgloss.Height(boardContent)

		// Panel content height matches the largest content, bounded by maxBodyPanelH
		panelH := min(max(minMainH, boardH), maxBodyPanelH)

		mainPanel := st.panel.Width(mainInnerW).Height(panelH).Render(mainContent)
		boardPanel := st.panel.Width(leaderboardInW).Height(panelH).Render(boardContent)
		body := lipgloss.JoinHorizontal(lipgloss.Top, mainPanel, strings.Repeat(" ", panelGap), boardPanel)

		// Remaining space for summary
		availForSummary := max(3, availForPanels-(panelH+2)-2)
		summaryContent := renderRoundSummary(r, st, m.lastRound, m.sessionID, availForSummary)
		summaryH := min(lipgloss.Height(summaryContent), availForSummary)
		summary := st.panel.Width(max(10, contentW-4)).Height(summaryH).Render(summaryContent)

		page := []string{topBlock, body, summary}
		return lipgloss.JoinVertical(lipgloss.Center, page...)
	}

	// Medium / Narrow layout (< sideBySideMinW)
	mainInnerW := max(10, contentW-4)
	barW := min(max(mainInnerW-2, minShareBarW), maxShareBarW)
	mainContent := lipgloss.JoinVertical(lipgloss.Center, choices, renderShareBar(r, st, m.dist, m.total, barW))
	mainPanel := st.panel.Width(mainInnerW).Render(mainContent)

	usedForMain := lipgloss.Height(mainPanel)
	availBelow := max(4, availForPanels-usedForMain)

	halfRenderedW := (contentW - panelGap) / 2
	halfInnerW := halfRenderedW - 4
	if halfInnerW >= 16 {
		// Place Leaderboard and Last Round side-by-side in bottom row
		bottomH := max(3, availBelow-2)
		boardContent := renderLeaderboard(st, m.leaderboard, m.sessionID, bottomH)
		summaryContent := renderRoundSummary(r, st, m.lastRound, m.sessionID, bottomH)

		rowH := min(max(lipgloss.Height(boardContent), lipgloss.Height(summaryContent)), bottomH)
		boardPanel := st.panel.Width(halfInnerW).Height(rowH).Render(boardContent)
		summaryPanel := st.panel.Width(halfInnerW).Height(rowH).Render(summaryContent)
		bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, boardPanel, strings.Repeat(" ", panelGap), summaryPanel)

		page := []string{topBlock, mainPanel, bottomRow}
		return lipgloss.JoinVertical(lipgloss.Center, page...)
	}

	// Very narrow stacked
	boardH := max(2, availBelow/2)
	boardContent := renderLeaderboard(st, m.leaderboard, m.sessionID, boardH)
	boardPanel := st.panel.Width(mainInnerW).Render(boardContent)

	sumH := max(2, availBelow-lipgloss.Height(boardPanel)-2)
	summaryContent := renderRoundSummary(r, st, m.lastRound, m.sessionID, sumH)
	summaryPanel := st.panel.Width(mainInnerW).Render(summaryContent)

	page := []string{topBlock, mainPanel, boardPanel, summaryPanel}
	return lipgloss.JoinVertical(lipgloss.Center, page...)
}

func (msg *roundEndMsg) sortedResults() []pickResult {
	out := make([]pickResult, 0, len(msg.results))
	for _, p := range msg.results {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		rank := func(o string) int {
			switch o {
			case "win":
				return 0
			case "lose":
				return 1
			default:
				return 2
			}
		}
		if rank(out[i].outcome) != rank(out[j].outcome) {
			return rank(out[i].outcome) < rank(out[j].outcome)
		}
		if out[i].user != out[j].user {
			return out[i].user < out[j].user
		}
		return out[i].sessionID < out[j].sessionID
	})
	return out
}
