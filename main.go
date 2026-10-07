// Rock-Paper-SSH: a rock-paper-scissors game served over SSH.
package main

import (
	"fmt"
	"math/rand/v2"
	"net"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/muesli/termenv"
)

const (
	port          = "2222"
	roundDuration = 3 * time.Second
)

var names = [3]string{"Stone", "Paper", "Scissors"}

var helpStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("240")).
	MarginTop(1)

// --- global round timer -----------------------------------------------------

var (
	timerMu  sync.Mutex
	roundEnd time.Time
)

func startRound(d time.Duration) {
	timerMu.Lock()
	roundEnd = time.Now().Add(d)
	timerMu.Unlock()
}

func timeLeft() time.Duration {
	timerMu.Lock()
	defer timerMu.Unlock()
	return time.Until(roundEnd)
}

// roundLoop ends the round for everyone exactly once, then starts the next.
func roundLoop() {
	for {
		time.Sleep(200 * time.Millisecond)
		if timeLeft() <= 0 {
			picks, dist, total := snapshotChoices()

			// The system's winning choice — random index, different every round.
			winner := rand.IntN(len(names))

			broadcast(roundEndMsg{
				winner: winner, // arbitrary data: the system's winning index
				picks:  picks,  // session id -> pick
				dist:   dist,   // % of users per choice, out of 100
				total:  total,  // how many users picked
			})

			startRound(roundDuration)
			resetChoices()
		}
	}
}

// --- global choice registry -------------------------------------------------

// pick is one user's locked-in choice. The registry is keyed by session id so
// two connections from the same username don't overwrite each other.
type pick struct {
	user string // display name (the SSH username)
	idx  int    // choice index (0..2)
}

var (
	choiceMu    sync.Mutex
	userChoices = map[string]pick{} // session id -> pick
)

func lockChoice(sessionID, user string, choiceIdx int) {
	choiceMu.Lock()
	userChoices[sessionID] = pick{user: user, idx: choiceIdx}
	choiceMu.Unlock()
}

func snapshotChoices() (map[string]pick, [3]float64, int) {
	choiceMu.Lock()
	snapshot := make(map[string]pick, len(userChoices))
	var counts [3]int
	for id, p := range userChoices {
		snapshot[id] = p
		counts[p.idx]++
	}
	choiceMu.Unlock()

	// what fraction of users picked each choice, out of 100%
	var dist [3]float64
	total := len(snapshot)
	if total > 0 {
		for i := range counts {
			dist[i] = float64(counts[i]) / float64(total) * 100
		}
	}
	return snapshot, dist, total
}

func resetChoices() {
	choiceMu.Lock()
	userChoices = map[string]pick{}
	choiceMu.Unlock()
}

// --- broadcast fan-out ------------------------------------------------------

// one channel per connected session
var (
	subMu sync.Mutex
	subs  = map[chan tea.Msg]struct{}{}
)

func broadcast(m tea.Msg) {
	subMu.Lock()
	defer subMu.Unlock()
	for ch := range subs {
		select {
		case ch <- m:
		default: // drop for sessions that aren't reading fast enough
		}
	}
}

// --- bubbletea UI -----------------------------------------------------------

// roundEndMsg carries the round's outcome — same data delivered to everyone.
type roundEndMsg struct {
	winner int             // the system's winning choice index
	picks  map[string]pick // session id -> pick
	dist   [3]float64      // % of users per choice, out of 100
	total  int             // how many users picked
}

type tickMsg time.Time

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	user      string // SSH username, for display
	sessionID string // unique per connection — the registry key
	choice    int
	winner    int
	picks     map[string]pick
	dist      [3]float64
	total     int
	renderer  *lipgloss.Renderer // uses this SSH client's color capabilities
}

func (m model) Init() tea.Cmd { return tickEvery() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case roundEndMsg: // all users receive the same round outcome at the same time
		m.winner = msg.winner
		m.picks = msg.picks
		m.dist = msg.dist
		m.total = msg.total
		return m, nil

	case tickMsg:
		return m, tickEvery()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.choice = (m.choice + 1) % len(names)
			return m, nil
		}
	}
	return m, nil
}

func (m model) View() string {
	s := fmt.Sprintf("⏳ %ds left in round\n\n", int(timeLeft().Seconds())+1)
	s += renderChoices(m.renderer, m.choice)
	s += fmt.Sprintf("Your current choice is, %s\n", names[m.choice])
	lockChoice(m.sessionID, m.user, m.choice)
	if m.picks != nil {
		s += fmt.Sprintf("\nROUND ENDED — system picked %s as the winner\n", names[m.winner])

		s += fmt.Sprintf("\nWhat users picked (%d pickers):\n", m.total)
		for i, pct := range m.dist {
			s += fmt.Sprintf("  %s : %5.1f%%\n", names[i], pct)
		}

		s += "\nThis round's picks:\n"
		for _, p := range m.picks {
			verdict := ""
			if p.idx == m.winner {
				verdict = " 🎉"
			}
			s += fmt.Sprintf("  %s chose %s%s\n", p.user, names[p.idx], verdict)
		}
	}
	return s + helpStyle.Render("tab to choose · ctrl+c to quit") + "\n"
}

// --- ssh wiring -------------------------------------------------------------

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	log.Info(s.User())
	r := bubbletea.MakeRenderer(s) // color profile of THIS client's terminal
	return model{
		user:      s.User(),
		sessionID: s.Context().SessionID(), // unique per connection
		choice:    rand.IntN(len(names)),
		renderer:  r,
	}, []tea.ProgramOption{tea.WithAltScreen()}
}

// programHandler builds each session's program and subscribes it to broadcasts.
func programHandler(s ssh.Session) *tea.Program {
	m, opts := teaHandler(s)
	p := tea.NewProgram(m, append(opts, bubbletea.MakeOptions(s)...)...)

	ch := make(chan tea.Msg, 16)
	subMu.Lock()
	subs[ch] = struct{}{}
	subMu.Unlock()

	go func() {
		for msg := range ch {
			p.Send(msg)
		}
	}()
	return p
}

func main() {
	startRound(roundDuration)
	go roundLoop()

	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort("localhost", port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithMiddleware(bubbletea.MiddlewareWithProgramHandler(programHandler, termenv.Ascii)),
	)
	if err != nil {
		log.Fatal("Could not start server", "error", err)
	}

	log.Info("Starting SSH server", "port", port)
	log.Fatal(s.ListenAndServe())
}
