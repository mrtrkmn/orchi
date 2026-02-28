import React from 'react';
import { Link } from 'react-router-dom';
import { useEventContext } from '../hooks/useEventContext';

/**
 * Event landing page — displayed when a user visits <eventname>.cyberorch.com.
 *
 * Shows event details, status, registration, and navigation to challenges,
 * scoreboard, and lab for the active event.
 */
export const EventPage: React.FC = () => {
  const { slug, event, isLoading, error, isEventContext } = useEventContext();

  if (!isEventContext) {
    return (
      <div className="event-page">
        <div className="page-header">
          <h1>
            <span className="accent">{'>'}</span> Events
          </h1>
        </div>
        <div className="card">
          <p className="text-muted">
            Visit <code>&lt;event-name&gt;.cyberorch.com</code> to access a
            specific event, or browse available events below.
          </p>
        </div>
        <EventList />
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="event-page">
        <div className="loading-container">
          <div className="loading-spinner" />
          <p className="text-muted">Loading event...</p>
        </div>
      </div>
    );
  }

  if (error || !event) {
    return (
      <div className="event-page">
        <div className="page-header">
          <h1>
            <span className="accent">{'>'}</span> Event Not Found
          </h1>
        </div>
        <div className="card">
          <p className="text-muted">
            No event found for <code>{slug}</code>. Check the URL or contact
            your event organizer.
          </p>
          <a href="https://cyberorch.com" className="btn btn-secondary">
            ← Back to Orchi
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="event-page">
      <div className="page-header">
        <h1>
          <span className="accent">{'>'}</span> {event.name}
        </h1>
        <div className="event-meta">
          <span className={`status-badge status-${event.status}`}>
            {event.status}
          </span>
          <span className="text-muted">
            {new Date(event.start_time).toLocaleDateString()} —{' '}
            {new Date(event.end_time).toLocaleDateString()}
          </span>
        </div>
      </div>

      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-value">{event.team_count}</div>
          <div className="stat-label">
            Teams ({event.max_teams} max)
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{event.challenges_count}</div>
          <div className="stat-label">Challenges</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">{event.vpn_enabled ? '✓' : '—'}</div>
          <div className="stat-label">VPN Access</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">
            {event.browser_access ? '✓' : '—'}
          </div>
          <div className="stat-label">Browser Desktop</div>
        </div>
      </div>

      <div className="dashboard-grid">
        <div className="card">
          <h3>Get Started</h3>
          <p className="text-muted">
            Register or log in, create or join a team, and start solving
            challenges.
          </p>
          <div className="card-actions">
            <Link to="/login" className="btn btn-primary">
              Sign In
            </Link>
            <Link to="/register" className="btn btn-secondary">
              Register
            </Link>
          </div>
        </div>

        <div className="card">
          <h3>Challenges</h3>
          <p className="text-muted">
            Browse {event.challenges_count} challenge
            {event.challenges_count !== 1 ? 's' : ''} across categories.
          </p>
          <Link to="/challenges" className="btn btn-secondary">
            View Challenges →
          </Link>
        </div>

        <div className="card">
          <h3>Scoreboard</h3>
          <p className="text-muted">
            Live scoreboard with real-time updates.
          </p>
          <Link to="/scoreboard" className="btn btn-secondary">
            View Scoreboard →
          </Link>
        </div>

        {event.browser_access && (
          <div className="card">
            <h3>Lab Environment</h3>
            <p className="text-muted">
              Access your lab VM directly from the browser.
            </p>
            <Link to="/lab" className="btn btn-secondary">
              Open Lab →
            </Link>
          </div>
        )}
      </div>
    </div>
  );
};

/**
 * Lists all available events (shown on the main domain).
 */
const EventList: React.FC = () => {
  // In production this would use React Query with api.get('/events')
  return (
    <div className="event-list">
      <div className="card">
        <h3>Available Events</h3>
        <p className="text-muted">
          No events currently available. Check back soon or ask your organizer
          for the event URL.
        </p>
      </div>
    </div>
  );
};
