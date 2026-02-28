import React, { useState } from 'react';
import type { Challenge } from '../types/api';

/**
 * Challenges page — browse and filter CTF challenges.
 *
 * Features:
 * - Category filter tabs
 * - Difficulty filter
 * - Search by name/tag
 * - Challenge cards with solve status
 * - Flag submission modal
 */
export const ChallengesPage: React.FC = () => {
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [searchQuery, setSearchQuery] = useState('');

  // Example challenge data — in production, fetched via React Query
  const challenges: Challenge[] = [];
  const categories = ['all', 'web', 'crypto', 'forensics', 'pwn', 'misc'];

  const filteredChallenges = challenges.filter((c) => {
    const matchesCategory = selectedCategory === 'all' || c.category === selectedCategory;
    const matchesSearch =
      searchQuery === '' ||
      c.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      c.tags.some((t) => t.toLowerCase().includes(searchQuery.toLowerCase()));
    return matchesCategory && matchesSearch;
  });

  return (
    <div className="challenges-page">
      <div className="page-header">
        <h1>
          <span className="accent">{'>'}</span> Challenges
        </h1>
      </div>

      <div className="filters">
        <div className="category-tabs">
          {categories.map((cat) => (
            <button
              key={cat}
              className={`tab ${selectedCategory === cat ? 'tab-active' : ''}`}
              onClick={() => setSelectedCategory(cat)}
            >
              {cat.charAt(0).toUpperCase() + cat.slice(1)}
            </button>
          ))}
        </div>

        <input
          type="text"
          className="search-input"
          placeholder="Search challenges..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
        />
      </div>

      <div className="challenge-grid">
        {filteredChallenges.length === 0 ? (
          <div className="empty-state">
            <p className="text-muted">
              No challenges available. Join an active event to see challenges.
            </p>
          </div>
        ) : (
          filteredChallenges.map((challenge) => (
            <ChallengeCard key={challenge.id} challenge={challenge} />
          ))
        )}
      </div>
    </div>
  );
};

interface ChallengeCardProps {
  challenge: Challenge;
}

const ChallengeCard: React.FC<ChallengeCardProps> = ({ challenge }) => {
  const difficultyColors: Record<string, string> = {
    easy: '#22c55e',
    medium: '#f59e0b',
    hard: '#ef4444',
    insane: '#a855f7',
  };

  return (
    <div className={`challenge-card ${challenge.solved ? 'solved' : ''}`}>
      <div className="challenge-header">
        <span className="challenge-category">{challenge.category}</span>
        <span
          className="challenge-difficulty"
          style={{ color: difficultyColors[challenge.difficulty] }}
        >
          {challenge.difficulty}
        </span>
      </div>
      <h3 className="challenge-name">{challenge.name}</h3>
      <div className="challenge-footer">
        <span className="challenge-points">{challenge.points} pts</span>
        <span className="challenge-solves">{challenge.solved_by} solves</span>
      </div>
      {challenge.solved && <div className="solved-badge">✓ Solved</div>}
    </div>
  );
};
