package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
)

const port = "2222"

// TIMER SETUP START

var timerMu sync.Mutex
var roundEnd time.Time // zero value = "no round running"

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

func roundOver() bool {
	return timeLeft() <= 0
}

// broadcast plumbing: one channel per connected session
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
		default: // skip sessions that aren't reading fast enough
		}
	}
}

// roundLoop runs on the server: detects expiry ONCE and tells everyone.
func roundLoop() {
	for {
		time.Sleep(200 * time.Millisecond)
		if roundOver() {
			broadcast(roundEndMsg{})
			startRound(roundDuration) // next round starts for everyone simultaneously
		}
	}
}

// TIMER SETUP END

const roundDuration = 30 * time.Second

// roundEndMsg is sent by the server to every client when the round expires.
type roundEndMsg struct{}

type tickMsg time.Time

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	messages []string
	input    textinput.Model
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, tickEvery())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case roundEndMsg: // all clients receive this in the same instant
		m.messages = append(m.messages, "⏰ Round ended! New round started.")
		return m, nil

	case tickMsg:
		return m, tickEvery()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			text := strings.TrimSpace(m.input.Value())
			if text != "" {
				m.messages = append(m.messages, text)
				m.input.SetValue("")
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() string {
	s := fmt.Sprintf("⏳ %ds left in round\n\n", int(timeLeft().Seconds())+1)
	for _, msg := range m.messages {
		s += msg + "\n"
	}
	return s + m.input.View() + "\n\n(enter to submit, ctrl+c to quit)\n"
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	log.Info(s.User())
	ti := textinput.New()
	ti.Placeholder = "type here…"
	ti.Focus()
	ti.CharLimit = 50
	ti.Width = 40
	return model{input: ti}, []tea.ProgramOption{tea.WithAltScreen()}
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
			p.Send(msg) // arrives in that session's Update
		}
	}()
	return p
}

func main() {
	startRound(roundDuration) // global round starts when the server starts
	go roundLoop()            // server watches the clock and broadcasts expiry

	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort("localhost", port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithMiddleware(bubbletea.MiddlewareWithProgramHandler(programHandler, 0)),
	)
	if err != nil {
		log.Fatal("Could not start server", "error", err)
	}

	log.Info("Starting SSH server", "port", port)
	log.Fatal(s.ListenAndServe())
}
