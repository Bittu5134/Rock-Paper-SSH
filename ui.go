package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ui.go — full-page responsive layout: a centered bordered frame, two panes
// (choices + share bar, leaderboard), a vibrant solid-color share bar and
// height-aware trimming. Everything renders through the session renderer so
// colors degrade to each SSH client's supported color profile.

// --- vibrant palette --------------------------------------------------------

const (
	colTitle  = "#FF2FD6" // hot magenta
	colAccent = "#22F2FF" // electric cyan
	colWarn   = "#F9E71C" // bright yellow
	colBad    = "#FF4D6D" // hot red
	colGood   = "#3DF273" // neon green
	colDim    = "#737373" // dim gray
)

// per-choice colors: one saturated color per choice, shared by the icons,
// the share bar segments and the winner highlight.
const (
	colStone    = "#FFA657" // vivid orange
	colPaper    = colWarn   // bright yellow
	colScissors = colBad    // hot red
)

// --- layout constants (tune by eye) -----------------------------------------

const (
	frameMargin    = 6  // outer margin: border (2) + frame padding (4)
	maxContentW    = 90 // cap for very wide terminals
	minContentW    = 50 // enough room for the three choice icons
	wideBreakpoint = 88 // >= this content width: panels sit side by side
	leaderboardInW = 30 // leaderboard content width (wide mode)
	panelGap       = 2  // gap between side-by-side panels
	minShareBarW   = 20
	maxShareBarW   = 60
)

// uiStyles holds the pre-built styles for one session's renderer.
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

// choiceColor maps a choice index to its vibrant color.
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

// --- components -------------------------------------------------------------

// renderTimer draws the round countdown: a bar plus seconds, sized to the
// available width, shifting cyan → yellow → red as the round runs out.
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

// renderShareBar draws one horizontal bar of `width` cells split into three
// solid vibrant segments proportional to dist (percentages summing to ~100).
func renderShareBar(r *lipgloss.Renderer, st *uiStyles, dist [3]float64, total int, width int) string {
	head := st.panelHead.Render("PICKS ACROSS THE BOARD")
	if total == 0 || width < 10 {
		return lipgloss.JoinVertical(lipgloss.Left, head, st.label.Render("no picks yet — waiting for players"))
	}

	w0 := int(dist[0] / 100 * float64(width))
	w1 := int(dist[1] / 100 * float64(width))
	w2 := width - w0 - w1 // remainder keeps the bar exactly `width` cells

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

// renderLeaderboard renders the sorted board, capped to maxRows with a
// "+k more players" footer when trimmed.
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

// renderRoundSummary shows the last round's outcome: the system's winning
// choice and every player's result with their points delta.
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

// --- responsive full page ---------------------------------------------------

// pageOpts controls the progressive trimming used to fit short terminals.
type pageOpts struct {
	showSubtitle bool
	showSummary  bool
	hideBoard    bool
	lbRows       int
}

// renderFullPage assembles the page, fits it to the client's terminal, wraps
// it in the outer border and centers it.
func renderFullPage(m model) string {
	r, st := m.renderer, m.styles

	w, h := m.width, m.height
	if w <= 0 {
		w = 80 // first frame, before the initial WindowSizeMsg
	}
	if h <= 0 {
		h = 24
	}

	contentW := min(max(w-frameMargin, minContentW), maxContentW)
	contentH := h - frameMargin
	wide := contentW >= wideBreakpoint
	nRows := max(contentH-24, 5) // room for chrome + choices + share bar

	// Progressive trim ladder: drop optional chrome until the page fits.
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

// assemblePage builds one layout variant: side-by-side when wide, stacked
// panels when narrow.
func assemblePage(r *lipgloss.Renderer, st *uiStyles, m model, o pageOpts, contentW int, wide bool) string {
	// header
	parts := []string{st.title.Render("ROCK  ·  PAPER  ·  SCISSORS")}
	if o.showSubtitle {
		parts = append(parts, st.subtitle.Render("SSH battle — beat the board, take their points"))
	}
	header := lipgloss.JoinVertical(lipgloss.Center, parts...)

	timer := renderTimer(r, st, contentW)
	choices := renderChoices(r, m.choice)

	// Panel sizing: lipgloss Width() includes padding but excludes the border,
	// so a panel's total width is Width(...) + 2. mainW is the Width() argument.
	mainW := contentW - 2
	if wide && !o.hideBoard {
		mainW = contentW - (leaderboardInW + 4) - panelGap - 2
	}
	barW := min(max(mainW-2, minShareBarW), maxShareBarW)

	mainContent := lipgloss.JoinVertical(lipgloss.Left, choices, "", renderShareBar(r, st, m.dist, m.total, barW))

	// body: side-by-side with equal panel heights, or stacked full width
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

// sortedResults returns the round's outcomes as a deterministic slice.
func (msg *roundEndMsg) sortedResults() []pickResult {
	out := make([]pickResult, 0, len(msg.results))
	for _, p := range msg.results {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		// winners first, then losers, then draws; alphabetical within a group
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
