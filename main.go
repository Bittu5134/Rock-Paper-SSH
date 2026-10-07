// Rock-Paper-SSH: a rock-paper-scissors battle royale served over SSH.
package main

import (
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
	roundDuration = 10 * time.Second
)

var names = [3]string{"Stone", "Paper", "Scissors"}

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

			// Score the round: winners take the losers' points.
			results := scoreRound(winner, picks)

			broadcast(roundEndMsg{
				winner:      winner,           // the system's winning index
				results:     results,          // session id -> outcome + delta
				dist:        dist,             // % of users per choice, out of 100
				total:       total,            // how many users picked
				leaderboard: getLeaderboard(), // fresh, sorted
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

// --- bubbletea glue ---------------------------------------------------------

// roundEndMsg carries the fully scored round — same data delivered to everyone
// at the same instant.
type roundEndMsg struct {
	winner      int                   // the system's winning choice index
	results     map[string]pickResult // session id -> outcome + points delta
	dist        [3]float64            // % of users per choice, out of 100
	total       int                   // how many users picked
	leaderboard leaderboard           // sorted, most points first
}

type tickMsg time.Time

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	user        string             // SSH username, for display
	sessionID   string             // unique per connection — the registry key
	choice      int                // currently selected choice (tab cycles)
	width       int                // this client's terminal width
	height      int                // this client's terminal height
	renderer    *lipgloss.Renderer // uses this SSH client's color capabilities
	styles      *uiStyles
	lastRound   *roundEndMsg // most recent scored round, if any
	leaderboard leaderboard  // current global board
	dist        [3]float64   // share of each choice from the last round
	total       int
}

func (m model) Init() tea.Cmd { return tickEvery() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case roundEndMsg: // all users receive the same scored round at the same time
		round := msg
		m.lastRound = &round
		m.leaderboard = msg.leaderboard
		m.dist = msg.dist
		m.total = msg.total
		return m, nil

	case tickMsg:
		return m, tickEvery()

	case tea.WindowSizeMsg: // initial size + every live resize
		m.width, m.height = msg.Width, msg.Height
		return m, nil

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
	// keep the global registry in sync with what this session has selected
	lockChoice(m.sessionID, m.user, m.choice)
	return renderFullPage(m)
}

// --- ssh wiring -------------------------------------------------------------

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	log.Info(s.User())
	r := bubbletea.MakeRenderer(s) // color profile of THIS client's terminal
	return model{
		user:        s.User(),
		sessionID:   s.Context().SessionID(), // unique per connection
		choice:      rand.IntN(len(names)),
		renderer:    r,
		styles:      newStyles(r),
		leaderboard: getLeaderboard(),
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
