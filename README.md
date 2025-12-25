# Votigo

A voting app for Palm's Arcade Retro LAN. Works on modern devices and ancient browsers (IE 6-7, Netscape).

## Quick Start

```bash
# Build
go build -o votigo .

# Create a category
./votigo category create "Best Costume" --type single

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
votigo category list              # List all categories
votigo category create NAME       # Create category
votigo option add CATEGORY_ID NAME
votigo option list CATEGORY_ID
votigo open CATEGORY_ID           # Open voting
votigo close CATEGORY_ID          # Close voting
votigo results CATEGORY_ID        # Show results
votigo serve --port 5000 --admin-password PASS
```

## Cross-Compile

```bash
GOOS=windows GOARCH=amd64 go build -o votigo.exe .
GOOS=linux GOARCH=amd64 go build -o votigo-linux .
```
