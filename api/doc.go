// Package api implements the REST API gateway for the Orchi platform.
//
// The API gateway translates REST/JSON requests from the frontend SPA into
// gRPC calls to the backend services (Daemon, Store, Exercise). It handles
// authentication (JWT), authorization (RBAC), CORS, rate limiting, and
// WebSocket connections for live scoreboard updates.
package api
