# tiny tracker

A simple, beautiful goal tracking application with offline support and automatic sync.

## Features

- Track daily, weekly, and monthly goals
- Visual progress tracking with calendar views
- Full offline support with automatic background sync
- Cross-device synchronization
- Push notifications for goal reminders
- Mobile app support (iOS and Android via Capacitor)

## Authentication

tiny tracker requires a Google account to use. Your data is synced across all your devices.

### First Time Setup
1. Open the app
2. Click "Sign in with Google"
3. Authorize the app
4. Start tracking your goals!

### Offline Usage
The app works fully offline. Changes are automatically synced when you reconnect.

## Tech Stack

### Frontend
- Svelte 5
- TypeScript
- IndexedDB (via idb) for local storage
- Capacitor for mobile apps

### Backend
- Go
- PostgreSQL
- Google OAuth 2.0
- CRDT-based sync

## Documentation

- [Authentication and Sync Architecture](docs/architecture/auth-and-sync.md)

## Development

### Frontend

```bash
cd frontend
npm install
npm run dev
```

### Backend

```bash
cd backend
go run cmd/server/main.go
```

## License

MIT
