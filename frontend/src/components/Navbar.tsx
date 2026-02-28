import React from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuthStore } from '../hooks/useAuth';
import { getEventSlug } from '../hooks/useEventContext';

/**
 * Navigation bar component.
 *
 * Shows different links based on authentication state and user role.
 * When on an event subdomain, shows the event name and event-specific links.
 */
export const Navbar: React.FC = () => {
  const { user, isAuthenticated, logout } = useAuthStore();
  const navigate = useNavigate();
  const eventSlug = getEventSlug();

  const handleLogout = () => {
    logout();
    navigate('/');
  };

  return (
    <nav className="navbar">
      <div className="navbar-brand">
        <Link to="/" className="navbar-logo">
          <span className="accent">{'>'}</span> Orchi
          {eventSlug && (
            <span className="event-slug"> / {eventSlug}</span>
          )}
        </Link>
      </div>

      <div className="navbar-links">
        {isAuthenticated ? (
          <>
            <Link to="/dashboard" className="nav-link">Dashboard</Link>
            <Link to="/challenges" className="nav-link">Challenges</Link>
            <Link to="/scoreboard" className="nav-link">Scoreboard</Link>
            <Link to="/lab" className="nav-link">Lab</Link>
            {!eventSlug && (
              <Link to="/events" className="nav-link">Events</Link>
            )}
            {user?.role === 'admin' && (
              <Link to="/admin" className="nav-link nav-admin">Admin</Link>
            )}
            <div className="nav-user">
              <span className="nav-username">{user?.username}</span>
              <button onClick={handleLogout} className="btn btn-sm btn-secondary">
                Logout
              </button>
            </div>
          </>
        ) : (
          <>
            {!eventSlug && (
              <Link to="/events" className="nav-link">Events</Link>
            )}
            <Link to="/login" className="nav-link">Login</Link>
            <Link to="/register" className="btn btn-sm btn-primary">Register</Link>
          </>
        )}
      </div>
    </nav>
  );
};
