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
	frameMargin    = 6
	maxContentW    = 90
	minContentW    = 50
	wideBreakpoint = 88
	leaderboardInW = 30
	panelGap       = 2
	minShareBarW   = 20
	maxShareBarW   = 60
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
		panelHead: r.NewStyle().Bold(true).Foreground(lipgloss.Color(colAccent)).MarginBottom(1),
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
	head := st.panelHead.Render("PICKS ACROSS THE BOARD")
	if total == 0 || width < 10 {
		return lipgloss.JoinVertical(lipgloss.Left, head, st.label.Render("no picks yet — waiting for players"))
	}

	w0 := int(dist[0] / 100 * float64(width))
	w1 := int(dist[1] / 100 * float64(width))
	w2 := width - w0 - w1

	bar := r.NewStyle().Foreground(choiceColor(0)).Render(strings.Repeat("█", w0))
	bar += r.NewStyle().Foreground(choiceColor(1)).Render(strings.Repeat("█", w1))
	bar += r.NewStyle().Foreground(choiceColor(2)).Render(strings.Repeat("█", w2))

	legend := fmt.Sprintf("%s %s%%   %s %s%%   %s %s%%",
		r.NewStyle().Foreground(choiceColor(0)).Render("Stone"), fmtPct(dist[0]),
		r.NewStyle().Foreground(choiceColor(1)).Render("Paper"), fmtPct(dist[1]),
		r.NewStyle().Foreground(choiceColor(2)).Render("Scissors"), fmtPct(dist[2]))

	return lipgloss.JoinVertical(lipgloss.Left, head, bar, legend)
}

func fmtPct(p float64) string {
	return fmt.Sprintf("%5.1f", p)
}

func renderLeaderboard(st *uiStyles, board leaderboard, meSessionID string, maxRows int) string {
	head := st.panelHead.Render(fmt.Sprintf("LEADERBOARD  ·  %d players", len(board)))
	if len(board) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, head, st.label.Render("nobody has points yet"))
	}

	if maxRows < 1 {
		maxRows = 1
	}
	shown, hidden := board, 0
	if len(board) > maxRows {
		shown, hidden = board[:maxRows], len(board)-maxRows
	}

	rows := make([]string, 0, len(shown)+1)
	for rank, e := range shown {
		name := e.user
		if e.sessionID == meSessionID {
			name = st.you.Render(name + " (you)")
		}
		style := st.label
		if e.points > 0 || rank == 0 {
			style = st.win
		}
		rows = append(rows, fmt.Sprintf("%2d. %s %s", rank+1, name, style.Render(fmt.Sprintf("%4d pts", e.points))))
	}
	if hidden > 0 {
		rows = append(rows, st.label.Render(fmt.Sprintf("    +%d more players", hidden)))
	}
	return lipgloss.JoinVertical(lipgloss.Left, head, strings.Join(rows, "\n"))
}

func renderRoundSummary(r *lipgloss.Renderer, st *uiStyles, msg *roundEndMsg, meSessionID string) string {
	head := st.panelHead.Render("LAST ROUND")
	winner := r.NewStyle().Bold(true).Foreground(choiceColor(msg.winner)).Render(names[msg.winner])
	lines := []string{fmt.Sprintf("%s  %s", st.label.Render("WINNER"), winner)}

	for _, p := range msg.sortedResults() {
		mark, markStyle := "· draw", st.label
		name := p.user
		if p.sessionID == meSessionID {
			name = st.you.Render(name + " (you)")
		}
		switch {
		case p.idx == msg.winner:
			mark, markStyle = "▲ WIN", st.win
		case p.outcome == "lose":
			mark, markStyle = "▼ LOSE", st.lose
		}
		delta := ""
		if p.delta > 0 {
			delta = st.win.Render(fmt.Sprintf("+%d", p.delta))
		} else if p.delta < 0 {
			delta = st.lose.Render(fmt.Sprintf("%d", p.delta))
		}
		lines = append(lines, fmt.Sprintf("  %s %-8s %s %s", name, names[p.idx], markStyle.Render(mark), delta))
	}
	return lipgloss.JoinVertical(lipgloss.Left, head, strings.Join(lines, "\n"))
}

type pageOpts struct {
	showSubtitle bool
	showSummary  bool
	hideBoard    bool
	lbRows       int
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

	contentW := min(max(w-frameMargin, minContentW), maxContentW)
	contentH := h - frameMargin
	wide := contentW >= wideBreakpoint
	nRows := max(contentH-24, 5)

	attempts := []pageOpts{
		{showSubtitle: true, showSummary: true, lbRows: nRows},
		{showSubtitle: false, showSummary: true, lbRows: nRows},
		{showSubtitle: false, showSummary: false, lbRows: nRows},
		{showSubtitle: false, showSummary: false, lbRows: 3},
		{showSubtitle: false, showSummary: false, lbRows: 3, hideBoard: true},
	}

	var page string
	for _, o := range attempts {
		page = assemblePage(r, st, m, o, contentW, wide)
		if lipgloss.Height(page) <= contentH {
			break
		}
	}

	frame := r.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colAccent)).
		Padding(0, 2)

	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, frame.Render(page))
}

func assemblePage(r *lipgloss.Renderer, st *uiStyles, m model, o pageOpts, contentW int, wide bool) string {
	parts := []string{st.title.Render("ROCK  ·  PAPER  ·  SCISSORS")}
	if o.showSubtitle {
		parts = append(parts, st.subtitle.Render("SSH battle — beat the board, take their points"))
	}
	parts = append(parts, st.help.Render(m.hint))
	header := lipgloss.JoinVertical(lipgloss.Center, parts...)

	timer := renderTimer(r, st, contentW)
	choices := renderChoices(r, m.choice)

	mainW := contentW - 2
	if wide && !o.hideBoard {
		mainW = contentW - (leaderboardInW + 4) - panelGap - 2
	}
	barW := min(max(mainW-2, minShareBarW), maxShareBarW)

	mainContent := lipgloss.JoinVertical(lipgloss.Left, choices, "", renderShareBar(r, st, m.dist, m.total, barW))

	var body string
	switch {
	case o.hideBoard:
		body = st.panel.Width(mainW).Render(mainContent)
	case wide:
		boardContent := renderLeaderboard(st, m.leaderboard, m.sessionID, o.lbRows)
		panelH := max(lipgloss.Height(mainContent), lipgloss.Height(boardContent))
		mainPanel := st.panel.Width(mainW).Height(panelH).Render(mainContent)
		boardPanel := st.panel.Width(leaderboardInW + 2).Height(panelH).Render(boardContent)
		body = lipgloss.JoinHorizontal(lipgloss.Top, mainPanel, strings.Repeat(" ", panelGap), boardPanel)
	default:
		boardContent := renderLeaderboard(st, m.leaderboard, m.sessionID, o.lbRows)
		mainPanel := st.panel.Width(mainW).Render(mainContent)
		boardPanel := st.panel.Width(mainW).Render(boardContent)
		body = lipgloss.JoinVertical(lipgloss.Left, mainPanel, "", boardPanel)
	}

	page := []string{header, "", timer, "", body}

	if o.showSummary && m.lastRound != nil {
		summary := st.panel.Width(contentW - 2).Render(renderRoundSummary(r, st, m.lastRound, m.sessionID))
		page = append(page, "", summary)
	}

	page = append(page, "", st.help.Render("tab to pick · ctrl+c to quit"))

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
