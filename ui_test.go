package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderFullPage(t *testing.T) {
	r := lipgloss.DefaultRenderer()
	st := newStyles(r)

	m := model{
		user:        "TestUser",
		sessionID:   "session1",
		hint:        "A test hint",
		choice:      0,
		renderer:    r,
		styles:      st,
		leaderboard: leaderboard{{user: "Alice", points: 10, sessionID: "s1"}},
		width:       88,
		height:      26,
	}

	// 1. Initial state (waiting for first round) on standard 88x26
	out := renderFullPage(m)
	if strings.Contains(out, "TERMINAL TOO SMALL") {
		t.Fatalf("88x26 should not show TERMINAL TOO SMALL, got:\n%s", out)
	}
	if !strings.Contains(out, "PICKS ACROSS THE BOARD") {
		t.Errorf("expected PICKS ACROSS THE BOARD box to always be on screen")
	}
	if !strings.Contains(out, "LEADERBOARD") {
		t.Errorf("expected LEADERBOARD box to always be on screen")
	}
	if !strings.Contains(out, "LAST ROUND") {
		t.Errorf("expected LAST ROUND box to always be on screen")
	}

	// 2. Populated roundEndMsg on 88x26
	m.lastRound = &roundEndMsg{
		serverChoice: 1,
		results: map[string]pickResult{
			"session1": {user: "TestUser", sessionID: "session1", idx: 0, outcome: "lose", delta: -5},
			"session2": {user: "Alice", sessionID: "session2", idx: 2, outcome: "win", delta: 10},
		},
	}
	outWithRound := renderFullPage(m)
	if strings.Contains(outWithRound, "TERMINAL TOO SMALL") {
		t.Fatalf("88x26 with round results should not show TERMINAL TOO SMALL, got:\n%s", outWithRound)
	}
	if !strings.Contains(outWithRound, "PICKS ACROSS THE BOARD") {
		t.Errorf("expected PICKS ACROSS THE BOARD box")
	}
	if !strings.Contains(outWithRound, "LEADERBOARD") {
		t.Errorf("expected LEADERBOARD box")
	}
	if !strings.Contains(outWithRound, "LAST ROUND") {
		t.Errorf("expected LAST ROUND box")
	}

	// 3. Small screen size warning check
	smallSizes := [][2]int{
		{70, 26}, // too narrow
		{88, 20}, // too short
		{50, 15}, // both
	}
	for _, sz := range smallSizes {
		mSmall := m
		mSmall.width = sz[0]
		mSmall.height = sz[1]
		smallOut := renderFullPage(mSmall)
		if !strings.Contains(smallOut, "TERMINAL TOO SMALL") {
			t.Errorf("expected TERMINAL TOO SMALL for size %dx%d", sz[0], sz[1])
		}
	}
}
