// Rock-Paper-SSH: a rock-paper-scissors game served over SSH.
package main

import (
	"fmt"
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
	roundDuration = 30 * time.Second
)

var choices = [3]string{"🪨", "📄", "✂️ "}

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
			broadcast(roundEndMsg{})
			startRound(roundDuration)
		}
	}
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

type roundEndMsg struct{}

type tickMsg time.Time

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	choice int
}

func (m model) Init() tea.Cmd { return tickEvery() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case roundEndMsg:
		// all users reach here at the same time — hook in round scoring etc.
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
	return s + helpStyle.Render("tab to choose · ctrl+c to quit") + "\n"
}

// --- ssh wiring ---------------------------------------------------------------

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	log.Info(s.User())
	return model{}, []tea.ProgramOption{tea.WithAltScreen()}
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
