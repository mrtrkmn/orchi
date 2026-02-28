import React, { useState, useCallback } from 'react';
import { useScoreboard } from '../hooks/useScoreboard';
import type { TeamScore, ScoreUpdateMessage } from '../types/api';

/**
 * Scoreboard page — live competition scoreboard with WebSocket updates.
 *
 * Features:
 * - Real-time score updates via WebSocket
 * - Top-3 podium display
 * - Full team ranking table
 * - Live feed of recent solves
 * - Connection status indicator
 */
export const ScoreboardPage: React.FC = () => {
  const [teams, setTeams] = useState<TeamScore[]>([]);
  const [recentSolves, setRecentSolves] = useState<ScoreUpdateMessage['data'][]>([]);

  // Example event ID — in production, from URL params
  const eventId = 'current-event';

  const handleScoreUpdate = useCallback((data: ScoreUpdateMessage['data']) => {
    setTeams((prev) => {
      const updated = prev.map((t) =>
        t.team_id === data.team_id
          ? { ...t, score: data.new_score, rank: data.new_rank }
          : t
      );
      return updated.sort((a, b) => a.rank - b.rank);
    });

    setRecentSolves((prev) => [data, ...prev.slice(0, 9)]);
  }, []);

  const { isConnected } = useScoreboard(eventId, handleScoreUpdate);

  const topThree = teams.slice(0, 3);

  return (
    <div className="scoreboard-page">
      <div className="page-header">
        <h1>
          <span className="accent">{'>'}</span> Scoreboard
        </h1>
        <div className={`ws-status ${isConnected ? 'connected' : 'disconnected'}`}>
          <span className="ws-dot" />
          {isConnected ? 'Live' : 'Connecting...'}
        </div>
      </div>

      {/* Top 3 Podium */}
      {topThree.length > 0 && (
        <div className="podium">
          {topThree.map((team, index) => (
            <div key={team.team_id} className={`podium-place place-${index + 1}`}>
              <div className="podium-rank">#{index + 1}</div>
              <div className="podium-name">{team.team_name}</div>
              <div className="podium-score">{team.score} pts</div>
            </div>
          ))}
        </div>
      )}

      {/* Score Table */}
      <div className="score-table">
        <table>
          <thead>
            <tr>
              <th>Rank</th>
              <th>Team</th>
              <th>Score</th>
              <th>Solves</th>
              <th>Last Solve</th>
            </tr>
          </thead>
          <tbody>
            {teams.length === 0 ? (
              <tr>
                <td colSpan={5} className="text-muted text-center">
                  No scores yet. The competition hasn&apos;t started or no teams have joined.
                </td>
              </tr>
            ) : (
              teams.map((team) => (
                <tr key={team.team_id} className={team.rank <= 3 ? 'top-rank' : ''}>
                  <td className="rank">#{team.rank}</td>
                  <td className="team-name">{team.team_name}</td>
                  <td className="score">{team.score}</td>
                  <td>{team.challenges_solved}</td>
                  <td className="text-muted">{team.last_solve_at || '—'}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Live Feed */}
      <div className="live-feed">
        <h3>Live Feed</h3>
        {recentSolves.length === 0 ? (
          <p className="text-muted">Waiting for solves...</p>
        ) : (
          <ul>
            {recentSolves.map((solve, i) => (
              <li key={i} className="feed-item">
                <span className="feed-team">{solve.team_name}</span>
                {' solved '}
                <span className="feed-challenge">{solve.challenge_name}</span>
                {' for '}
                <span className="feed-points">{solve.points} pts</span>
                {solve.first_blood && <span className="first-blood">🩸 First Blood!</span>}
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
};
