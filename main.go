package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
)

const port = "2222"

const (
	phaseAskName = iota
	phaseApp
)

type model struct {
	user  string
	count int
	phase int
	input textinput.Model
}

func (m model) Init() tea.Cmd { return textinput.Blink }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.phase == phaseApp {
				return m, tea.Quit
			}
		case "esc":
			if m.phase == phaseApp {
				m.phase = phaseAskName
				return m, nil
			}
		case "enter":
			if m.phase == phaseAskName {
				name := strings.TrimSpace(m.input.Value())
				if name == "" {
					return m, nil
				}
				m.user = name
				m.input.SetValue("")
				m.phase = phaseApp
				return m, nil
			}
		case "up", "k":
			if m.phase == phaseApp {
				m.count++
				return m, nil
			}
		case "down", "j":
			if m.phase == phaseApp {
				m.count--
				return m, nil
			}
		}
	}
	// everything else (typing, cursor, paste) goes to the text input
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.phase == phaseAskName {
		return "What's your name?\n\n" + m.input.View() + "\n\n(enter to confirm, ctrl+c to quit)\n"
	}
	return fmt.Sprintf("Hello, %s!\n\nCount: %d\n\n(Use up/down or j/k, q to quit, esc to change name)\n", m.user, m.count)
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	log.Info(s.User())
	ti := textinput.New()
	ti.Placeholder = "type your name…"
	ti.Focus()
	ti.CharLimit = 24
	ti.Width = 30
	return model{user: s.User(), phase: phaseAskName, input: ti}, []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort("localhost", port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithMiddleware(bubbletea.Middleware(teaHandler)),
	)
	if err != nil {
		log.Fatal("Could not start server", "error", err)
	}

	log.Info("Starting SSH server", "port", port)
	log.Fatal(s.ListenAndServe())
}
