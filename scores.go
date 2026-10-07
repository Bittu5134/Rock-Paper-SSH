package main

import (
	"sort"
	"sync"
)

const winBase = 25

func beats(i int) int    { return (i + 2) % len(names) }
func beatenBy(i int) int { return (i + 1) % len(names) }

type ledgerEntry struct {
	user   string
	points int
}

var (
	ledgerMu sync.Mutex
	ledger   = map[string]ledgerEntry{}
)

func ensurePlayer(sessionID, user string) {
	ledgerMu.Lock()
	if _, ok := ledger[sessionID]; !ok {
		ledger[sessionID] = ledgerEntry{user: user, points: 0}
	}
	ledgerMu.Unlock()
}

func removePlayer(sessionID string) {
	ledgerMu.Lock()
	delete(ledger, sessionID)
	ledgerMu.Unlock()
}

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
		if !isSessionActive(id) {
			continue
		}
		board = append(board, leaderEntry{sessionID: id, user: e.user, points: e.points})
	}
	ledgerMu.Unlock()
	sort.Slice(board, func(i, j int) bool {
		if board[i].points != board[j].points {
			return board[i].points > board[j].points
		}
		return board[i].user < board[j].user
	})
	return board
}

type pickResult struct {
	sessionID     string
	user          string
	idx           int
	delta         int
	outcome       string
	opponentCount int
}

func scoreRound(serverChoice int, picks map[string]pick) map[string]pickResult {
	results := make(map[string]pickResult, len(picks))
	winningChoice := beatenBy(serverChoice)
	losingChoice := beats(serverChoice)

	pool := 0
	var winners, losers []string
	for id, p := range picks {
		if !isSessionActive(id) {
			continue
		}
		ensurePlayer(id, p.user)
		res := pickResult{sessionID: id, user: p.user, idx: p.idx}
		switch p.idx {
		case winningChoice:
			res.outcome = "win"
			winners = append(winners, id)
		case losingChoice:
			res.outcome = "lose"
			losers = append(losers, id)
		default:
			res.outcome = "draw"
		}
		results[id] = res
	}

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
