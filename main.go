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

func roundLoop() {
	for {
		time.Sleep(200 * time.Millisecond)
		if timeLeft() <= 0 {
			picks, dist, total := snapshotChoices()

			winner := rand.IntN(len(names))

			results := scoreRound(winner, picks)

			broadcast(roundEndMsg{
				winner:      winner,
				results:     results,
				dist:        dist,
				total:       total,
				leaderboard: getLeaderboard(),
			})

			startRound(roundDuration)
			resetChoices()
		}
	}
}

type pick struct {
	user string
	idx  int
}

var (
	choiceMu    sync.Mutex
	userChoices = map[string]pick{}
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
		default:
		}
	}
}

var WordDict = map[string][]string{
	"adjectives": {
		"Cyber", "Shadow", "Neon", "Epic", "Silent", "Cosmic", "Toxic",
		"Frozen", "Golden", "Iron", "Quantum", "Savage", "Phantom",
		"Pixel", "Alpha", "Hyper", "Astral", "Chaos", "Dark", "Swift",
		"Fierce", "Brave", "Rapid", "Stealthy", "Wild", "Electric", "Rogue",
	},
	"nouns": {
		"Viper", "Ninja", "Wolf", "Dragon", "Falcon", "Ghost", "Titan",
		"Reaper", "Knight", "Panda", "Shark", "Phoenix", "Goblin",
		"Wizard", "Samurai", "Cobra", "Demon", "Glitch", "Nomad", "Legend",
		"Stalker", "Hacker", "Valkyrie", "Beast", "Hunter", "Mage", "Spartan",
	},
}

func randomUser() string {
	adjList := WordDict["adjectives"]
	nounList := WordDict["nouns"]

	randomAdj := adjList[rand.IntN(len(adjList))]
	randomNoun := nounList[rand.IntN(len(nounList))]

	return randomAdj + randomNoun
}

type roundEndMsg struct {
	winner      int
	results     map[string]pickResult
	dist        [3]float64
	total       int
	leaderboard leaderboard
}

type tickMsg time.Time

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	user        string
	sessionID   string
	choice      int
	width       int
	height      int
	renderer    *lipgloss.Renderer
	styles      *uiStyles
	lastRound   *roundEndMsg
	leaderboard leaderboard
	dist        [3]float64
	total       int
}

func (m model) Init() tea.Cmd { return tickEvery() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case roundEndMsg:
		round := msg
		m.lastRound = &round
		m.leaderboard = msg.leaderboard
		m.dist = msg.dist
		m.total = msg.total
		return m, nil

	case tickMsg:
		return m, tickEvery()

	case tea.WindowSizeMsg:
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
	lockChoice(m.sessionID, m.user, m.choice)
	return renderFullPage(m)
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	log.Info(s.User())
	r := bubbletea.MakeRenderer(s)
	return model{
		user:        randomUser(),
		sessionID:   s.Context().SessionID(),
		choice:      rand.IntN(len(names)),
		renderer:    r,
		styles:      newStyles(r),
		leaderboard: getLeaderboard(),
	}, []tea.ProgramOption{tea.WithAltScreen()}
}

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
