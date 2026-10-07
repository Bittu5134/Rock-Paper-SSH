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

var choices = [3]string{"🪨", "📄", "✂️ "}
var win_choice = rand.IntN(len(choices))

var helpStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("240")).
	MarginTop(1)

// --- global round timer -----------------------------------------------------

var (
	timerMu  sync.Mutex
	roundEnd time.Time
	roundNum int // monotonically increasing round counter
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

			// Bump the round number under timerMu so every user sees the
			// same, consistent value.
			timerMu.Lock()
			roundNum++
			n := roundNum
			timerMu.Unlock()

			// The system's winning choice — random, different every round.
			winner := choices[rand.IntN(len(choices))]

			broadcast(roundEndMsg{
				round:  n,      // arbitrary data: which round just ended
				winner: winner, // arbitrary data: the system's winning glyph
				picks:  snapshotChoices(),
			})

			startRound(roundDuration)
			resetChoices()
		}
	}
}

// --- global choice registry -------------------------------------------------

var (
	choiceMu    sync.Mutex
	userChoices = map[string]string{} // username -> locked-in glyph
)

func lockChoice(user, glyph string) {
	choiceMu.Lock()
	userChoices[user] = glyph
	choiceMu.Unlock()
}

func snapshotChoices() map[string]string {
	choiceMu.Lock()
	snapshot := make(map[string]string, len(userChoices))
	for user, glyph := range userChoices {
		snapshot[user] = glyph
	}
	choiceMu.Unlock()
	return snapshot
}

func resetChoices() {
	choiceMu.Lock()
	userChoices = map[string]string{}
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
	round  int               // which round just ended
	winner string            // the system's winning glyph for this round
	picks  map[string]string // username -> glyph
}

type tickMsg time.Time

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	user    string
	choice  int
	round   int
	winner  string
	picks   map[string]string
}

func (m model) Init() tea.Cmd { return tickEvery() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case roundEndMsg: // all users receive the same round outcome at the same time
		m.round = msg.round
		m.winner = msg.winner
		m.picks = msg.picks
		return m, nil

	case tickMsg:
		return m, tickEvery()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.choice = (m.choice + 1) % len(choices)
			return m, nil
		}
	}
	return m, nil
}

func (m model) View() string {
	s := fmt.Sprintf("⏳ %ds left in round\n\n", int(timeLeft().Seconds())+1)
	s += fmt.Sprintf("Your current choice is, %s\n", choices[m.choice])
	lockChoice(m.user, choices[m.choice])
	if m.picks != nil {
		s += fmt.Sprintf("\nROUND %d ENDED — system picked %s as the winner\n", m.round, m.winner)
		s += "\nThis round's picks:\n"
		for user, glyph := range m.picks {
			verdict := ""
			if glyph == m.winner {
				verdict = " 🎉"
			}
			s += fmt.Sprintf("  %s chose %s%s\n", user, glyph, verdict)
		}
	}
	return s + helpStyle.Render("tab to choose · enter to lock in · ctrl+c to quit") + "\n"
}

// --- ssh wiring ---------------------------------------------------------------

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	log.Info(s.User())
	return model{user: s.User(), choice: rand.IntN(len(choices))}, []tea.ProgramOption{tea.WithAltScreen()}
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
