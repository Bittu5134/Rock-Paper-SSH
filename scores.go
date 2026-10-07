package main

import (
	"sort"
	"sync"
)

// scores.go — the global points ledger and round scoring.
//
// Rules:
//   • the system rolls a winning choice W each round
//   • pick == W                → WIN:  you take a share of the losers' points
//   • pick == the choice W beats → LOSE: you give up 50% of your points
//                                   (never drops below 0)
//   • pick == the choice that beats W → DRAW: nothing happens
//   • winners split the taken pool equally; with an empty pool each winner
//     still banks +winBase so the first rounds aren't pointless (pun intended)

const winBase = 25 // points a winner banks even when the losers are broke

// choice i beats choice (i+2)%3: 0 stone beats 2 scissors, 2 beats 1 paper, 1 beats 0 stone.
func beats(i int) int    { return (i + 2) % len(names) }
func beatenBy(i int) int { return (i + 1) % len(names) }

// --- global ledger -----------------------------------------------------------

type ledgerEntry struct {
	user   string
	points int
}

var (
	ledgerMu sync.Mutex
	ledger   = map[string]ledgerEntry{} // session id -> entry
)

// ensurePlayer registers a player in the ledger (0 points) on first pick.
func ensurePlayer(sessionID, user string) {
	ledgerMu.Lock()
	if _, ok := ledger[sessionID]; !ok {
		ledger[sessionID] = ledgerEntry{user: user, points: 0}
	}
	ledgerMu.Unlock()
}

// leaderboard is a sorted snapshot of the ledger: most points first.
type leaderboard []leaderEntry

type leaderEntry struct {
	sessionID string
	user      string
	points    int
}

func getLeaderboard() leaderboard {
	ledgerMu.Lock()
	board := make(leaderboard, 0, len(ledger))
	for id, e := range ledger {
		board = append(board, leaderEntry{sessionID: id, user: e.user, points: e.points})
	}
	ledgerMu.Unlock()
	sort.Slice(board, func(i, j int) bool {
		if board[i].points != board[j].points {
			return board[i].points > board[j].points // most points first
		}
		return board[i].user < board[j].user
	})
	return board
}

// --- round scoring -----------------------------------------------------------

// pickResult is one player's outcome for a finished round.
type pickResult struct {
	sessionID     string
	user          string
	idx           int    // what they picked
	delta         int    // points won (+) or lost (-), 0 for draw
	outcome       string // "win" | "lose" | "draw"
	opponentCount int    // how many losers the winners collectively beat
}

// scoreRound applies the rules for a finished round, mutates the ledger,
// and returns the enriched picks (with deltas) plus the fresh leaderboard.
func scoreRound(winner int, picks map[string]pick) map[string]pickResult {
	results := make(map[string]pickResult, len(picks))
	loserChoice := beats(winner) // the choice the winner beats

	// pass 1: classify every pick and stake the losers
	pool := 0
	var winners, losers []string
	for id, p := range picks {
		ensurePlayer(id, p.user)
		res := pickResult{sessionID: id, user: p.user, idx: p.idx}
		switch {
		case p.idx == winner:
			res.outcome = "win"
			winners = append(winners, id)
		case p.idx == loserChoice:
			res.outcome = "lose"
			losers = append(losers, id)
		default:
			res.outcome = "draw" // picked the choice that beats the winner
		}
		results[id] = res
	}

	// pass 2: losers pay 50% of their points (floor 0) into the pool
	ledgerMu.Lock()
	for _, id := range losers {
		e := ledger[id]
		lost := e.points / 2
		e.points -= lost
		ledger[id] = e
		res := results[id]
		res.delta = -lost
		results[id] = res
		pool += lost
	}

	// pass 3: winners split the pool (+ base bonus), draws unchanged
	perWinner := 0
	if len(winners) > 0 {
		perWinner = pool/len(winners) + winBase
	}
	for _, id := range winners {
		e := ledger[id]
		e.points += perWinner
		ledger[id] = e
		res := results[id]
		res.delta = perWinner
		res.opponentCount = len(losers)
		results[id] = res
	}
	ledgerMu.Unlock()

	return results
}
