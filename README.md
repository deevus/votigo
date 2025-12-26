# Votigo

A voting app for Palms Arcade Retro LAN. Works on modern devices and ancient browsers (IE 6-7, Netscape).

## Quick Start

```bash
# Build
go build -o votigo .

# Create a poll
./votigo poll create "Best Costume" --type single

# Add options
./votigo option add 1 "Player One"
./votigo option add 1 "RetroGamer"

# Open voting
./votigo open 1

# Start web server
./votigo serve --admin-password yoursecret
```

Voters access: http://YOUR_IP:5000
Admin access: http://YOUR_IP:5000/admin (user: admin)

## Vote Types

- `single` - Pick one option
- `approval` - Pick any number of options
- `ranked` - Rank top N choices (use `--max-rank`)

## Commands

```bash
votigo poll list                  # List all polls
votigo poll create NAME           # Create poll
votigo option add POLL_ID NAME
votigo option list POLL_ID
votigo open POLL_ID               # Open voting
votigo close POLL_ID              # Close voting
votigo results POLL_ID            # Show results
votigo serve --port 5000 --admin-password PASS
```

## Cross-Compile

```bash
GOOS=windows GOARCH=amd64 go build -o votigo.exe .
GOOS=linux GOARCH=amd64 go build -o votigo-linux .
```
