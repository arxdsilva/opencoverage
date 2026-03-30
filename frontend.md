# Frontend Guide

## Overview

This repository includes a separate frontend entrypoint at `cmd/frontend`.

The frontend is a dark-theme dashboard inspired by observability UIs (SigNoz-like):

- project list
- latest comparison summary
- package-level comparison table
- recent runs table

## No User Authentication

The frontend does not require user login/auth.

How it works:

1. Browser calls frontend endpoints under `/api/*`.
2. Frontend server proxies to coverage-api (`/v1/*`).
3. Frontend injects `API_KEY_SECRET` server-side into the API key header.
4. API key is never entered by the user in the UI.

## Folder Structure

- `cmd/frontend/main.go` - frontend server and API proxy
- `cmd/frontend/web/index.html` - app shell
- `cmd/frontend/web/assets/styles.css` - dark theme styles
- `cmd/frontend/web/assets/app.js` - UI logic and API calls

## Configuration

Environment variables used by frontend:

- `FRONTEND_ADDR` (default `:8090`)
- `API_BASE_URL` (default `http://localhost:8080`)
- `API_KEY_HEADER` (default `X-API-Key`)
- `API_KEY_SECRET` (default `dev-local-key`)

## Run

### Run API

```bash
export DATABASE_URL="postgres://coverage:coverage@localhost:5433/coverage?sslmode=disable"
export API_KEY_SECRET="dev-local-key"
go run ./cmd/api
```

### Run Frontend

```bash
FRONTEND_ADDR=":8090" API_BASE_URL="http://localhost:8080" API_KEY_SECRET="dev-local-key" go run ./cmd/frontend
```

Open:

- `http://localhost:8090`

### Run With Docker Compose

```bash
make compose-up
```

Services started by compose:

- API on `http://localhost:8080`
- Frontend on `http://localhost:8090`

## Make Targets

- `make frontend-run`
- `make frontend-dev`

`frontend-dev` prints the two commands to run API and frontend in separate terminals.

## Heatmap and Group Visualization

The dashboard includes an interactive heatmap view of all projects accessible via the "Open Heatmap" button.

### Features

- **Group Organization**: Projects with an assigned group appear as balanced panels within the heatmap overlay
- **Responsive Layout**: Groups automatically reflow to fill the visible overlay area as panels
- **Color Coding**: Groups are color-coded by overall status (green for passing/up, red for failing/down, neutral for other states)
- **Per-Group Tiles**: Each group contains project tiles sized to fill the group's allocated space
- **Real-time Relayout**: Groups reflow on window resize to maintain optimal use of screen space

### Group Colors

Heatmap groups are colored based on the aggregate status of their projects:
- **Green (up/passed)**: Group has more passing projects or average coverage >= 80%
- **Red (down/failed)**: Group has more failing projects or average coverage < 80%
- **Neutral**: No projects with data, or mixed status

Ungrouped projects appear in an "Ungrouped" panel at the bottom of the heatmap.

## Exposed Frontend API Proxy Routes

The frontend server exposes unauthenticated GET routes for browser use:

- `GET /api/projects`
- `GET /api/projects/{projectId}/coverage-runs`
- `GET /api/projects/{projectId}/coverage-runs/latest-comparison`

These are proxied to:

- `GET /v1/projects`
- `GET /v1/projects/{projectId}/coverage-runs`
- `GET /v1/projects/{projectId}/coverage-runs/latest-comparison`
