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
		width:       80,
		height:      24,
	}

	// 1. Initial state (waiting for first round) on standard 80x24
	out := renderFullPage(m)
	if strings.Contains(out, "TERMINAL TOO SMALL") {
		t.Fatalf("80x24 should not show TERMINAL TOO SMALL, got:\n%s", out)
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

	// 2. Populated roundEndMsg on 80x24
	m.lastRound = &roundEndMsg{
		serverChoice: 1,
		results: map[string]pickResult{
			"session1": {user: "TestUser", sessionID: "session1", idx: 0, outcome: "lose", delta: -5},
			"session2": {user: "Alice", sessionID: "session2", idx: 2, outcome: "win", delta: 10},
		},
	}
	outWithRound := renderFullPage(m)
	if strings.Contains(outWithRound, "TERMINAL TOO SMALL") {
		t.Fatalf("80x24 with round results should not show TERMINAL TOO SMALL, got:\n%s", outWithRound)
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

	// 3. Small screen size warning check for dimensions below minTermW (45) or minTermH (16)
	smallSizes := [][2]int{
		{40, 24}, // too narrow (<45)
		{80, 14}, // too short (<16)
		{35, 10}, // both
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

	// 4. Responsive narrow / compact screen sizes
	narrowSizes := [][2]int{
		{60, 24},
		{50, 20},
		{70, 22},
	}
	for _, sz := range narrowSizes {
		mNarrow := m
		mNarrow.width = sz[0]
		mNarrow.height = sz[1]
		narrowOut := renderFullPage(mNarrow)
		if strings.Contains(narrowOut, "TERMINAL TOO SMALL") {
			t.Errorf("expected size %dx%d to render responsively without TOO SMALL warning", sz[0], sz[1])
		}
	}

	// 5. Expandable area test on taller/larger terminals
	t.Run("ExpandableAreaOnLargeTerminals", func(t *testing.T) {
		board := make(leaderboard, 20)
		results := make(map[string]pickResult)
		for i := 0; i < 20; i++ {
			u := string(rune('A' + i))
			board[i] = leaderEntry{user: "Player" + u, points: 100 - i, sessionID: "s" + u}
			results["s"+u] = pickResult{user: "Player" + u, sessionID: "s" + u, idx: i % 3, outcome: "win", delta: 5}
		}
		mLarge := m
		mLarge.leaderboard = board
		mLarge.lastRound = &roundEndMsg{
			serverChoice: 0,
			results:      results,
		}

		testSizes := [][2]int{
			{80, 24},
			{90, 30},
			{100, 40},
			{120, 60},
		}
		for _, sz := range testSizes {
			mLarge.width = sz[0]
			mLarge.height = sz[1]
			out := renderFullPage(mLarge)
			if strings.Contains(out, "TERMINAL TOO SMALL") {
				t.Fatalf("%dx%d should fit without warning", sz[0], sz[1])
			}
		}
	})
}
