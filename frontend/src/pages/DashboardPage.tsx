import React from 'react';
import { useAuthStore } from '../hooks/useAuth';

/**
 * Dashboard page — the authenticated user's home.
 *
 * Shows active event info, team overview, recent solves, and quick stats.
 */
export const DashboardPage: React.FC = () => {
  const { user } = useAuthStore();

  return (
    <div className="dashboard">
      <div className="page-header">
        <h1>
          <span className="accent">{'>'}</span> Dashboard
        </h1>
        <p className="text-muted">
          Welcome back, <span className="accent">{user?.username}</span>
        </p>
      </div>

      <div className="stats-grid">
        <div className="stat-card">
          <div className="stat-value">0</div>
          <div className="stat-label">Challenges Solved</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">0</div>
          <div className="stat-label">Total Points</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">—</div>
          <div className="stat-label">Current Rank</div>
        </div>
        <div className="stat-card">
          <div className="stat-value">0</div>
          <div className="stat-label">Active Events</div>
        </div>
      </div>

      <div className="dashboard-grid">
        <div className="card">
          <h3>Active Events</h3>
          <p className="text-muted">No active events. Check back soon or ask your organizer.</p>
        </div>

        <div className="card">
          <h3>Recent Solves</h3>
          <p className="text-muted">No challenges solved yet. Start competing!</p>
        </div>

        <div className="card">
          <h3>Team</h3>
          <p className="text-muted">
            {user?.team_id
              ? 'View your team details in the Teams section.'
              : 'Join an event and create or join a team to get started.'}
          </p>
        </div>
      </div>
    </div>
  );
};
