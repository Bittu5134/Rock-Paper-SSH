<div align="center">

# Rock · Paper · SSH

**A PvE multiplayer rock-paper-scissors arena that's played entirely over SSH.**  
*No client. No install. No account. Just connect. And play. [Read more...](https://bittu.dev/blog/rock-paper-ssh/)*

```yaml
ssh rps.bittu.dev -p 2222
```

<sub>IPv6-only host — works from most mobile networks & modern ISPs; some home Wi-Fi can't reach it</sub>
<br>

<img src="screenshots/game.png" width="850" alt="Rock Paper SSH gameplay" />

<br>

</div>

## How To Play

- Every round opens with a riddle hinting at the server's secret move.
- Win against the server and take **+25 pts**.
- The winning pool is funded by losers, who forfeit **50%** of their points.
- A leaderboard shows the live scores of everyone playing.
- Each round is 10 seconds long, so you need to be quick.


## Controls

- Pick Rock/Paper/Scissors using `Tab` or any other key.
- `Ctrl+C` to quit the game.


## Run Locally

Clone this Git repository locally and run these commands:
```bash
go run .
ssh localhost -p 2222
```

<sub>Configurable via `PORT` · `HOST` · `SSH_HOST_KEY_PATH`</sub>

<br>

## Built Using

[Wish](https://github.com/charmbracelet/wish) · [Bubble Tea](https://github.com/charmbracelet/bubbletea) · [Lip Gloss](https://github.com/charmbracelet/lipgloss)

<br>
