import React from 'react';
import { Link } from 'react-router-dom';

/**
 * Landing page for the Orchi CTF Platform.
 *
 * Displays hero section, feature highlights, and call-to-action buttons.
 * This is the public entry point — no authentication required.
 */
export const LandingPage: React.FC = () => {
  return (
    <div className="landing">
      <div className="hero">
        <div className="hero-content">
          <h1 className="hero-title">
            <span className="accent">{'>'}</span> Orchi CTF Platform
          </h1>
          <p className="hero-subtitle">
            Kubernetes-native Capture The Flag competition platform.
            Build skills. Break things. Learn security.
          </p>
          <div className="hero-actions">
            <Link to="/register" className="btn btn-primary">
              Get Started
            </Link>
            <Link to="/login" className="btn btn-secondary">
              Sign In
            </Link>
          </div>
        </div>
      </div>

      <div className="features">
        <div className="feature-grid">
          <div className="feature-card">
            <div className="feature-icon">🏴</div>
            <h3>CTF Challenges</h3>
            <p>
              Web exploitation, cryptography, forensics, reverse engineering,
              and more. Challenges for every skill level.
            </p>
          </div>

          <div className="feature-card">
            <div className="feature-icon">🖥️</div>
            <h3>Browser-Based Labs</h3>
            <p>
              Access your lab environment directly from the browser.
              No setup required — just start hacking.
            </p>
          </div>

          <div className="feature-card">
            <div className="feature-icon">📊</div>
            <h3>Live Scoreboard</h3>
            <p>
              Real-time scoring with WebSocket updates.
              Track your team&apos;s progress as it happens.
            </p>
          </div>

          <div className="feature-card">
            <div className="feature-icon">🔐</div>
            <h3>VPN Access</h3>
            <p>
              WireGuard VPN for direct network access to challenge
              infrastructure. Full control over your attack surface.
            </p>
          </div>

          <div className="feature-card">
            <div className="feature-icon">👥</div>
            <h3>Team Competition</h3>
            <p>
              Form teams, collaborate on challenges, and compete
              against others in real-time.
            </p>
          </div>

          <div className="feature-card">
            <div className="feature-icon">☸️</div>
            <h3>Cloud Native</h3>
            <p>
              Built on Kubernetes with isolated lab environments
              per team. Scales to hundreds of concurrent teams.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
};
