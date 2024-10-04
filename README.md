# Note Mixer Relay

A Nostr relay that mixes and anonymizes events using the Khatru framework.

## Features

- Event mixing and anonymization
- Configurable allowed event kinds
- Optional pubkey whitelisting
- Event rebroadcasting to other relays

## Prerequisites

- Go

## Installation

1. Clone the repository: `git clone https://github.com/gzuuus/note-mixer-relay.git && cd note-mixer-relay`
2. Copy the example environment file and edit it with your settings: `cp .env.example .env && nano .env`
3. Build the project: `go build -o note-mixer-relay`

## Configuration

Edit the `.env` file to configure the relay:

- `RELAY_NAME`: The name of your relay
- `RELAY_PUBKEY`: Your relay's public key
- `RELAY_PRIVATE_KEY`: Your relay's private key
- `RELAY_ICON`: URL to your relay's icon
- `RELAY_DESCRIPTION`: A brief description of your relay
- `ALLOWED_KINDS`: Comma-separated list of allowed event kinds
- `DB_PATH`: Path to the SQLite database file
- `PORT`: The port on which the relay will run
- `WHITELISTED_PUBKEYS`: Comma-separated list of whitelisted pubkeys (leave empty to allow all)
- `REBROADCAST_RELAYS`: Comma-separated list of relay URLs to rebroadcast events to

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the [MIT License](LICENSE).