import React, { useEffect } from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { Navbar } from './components/Navbar';
import { ProtectedRoute } from './components/ProtectedRoute';
import { LandingPage } from './pages/LandingPage';
import { LoginPage } from './pages/LoginPage';
import { RegisterPage } from './pages/RegisterPage';
import { DashboardPage } from './pages/DashboardPage';
import { ChallengesPage } from './pages/ChallengesPage';
import { ScoreboardPage } from './pages/ScoreboardPage';
import { LabPage } from './pages/LabPage';
import { EventPage } from './pages/EventPage';
import { useAuthStore } from './hooks/useAuth';
import { getEventSlug } from './hooks/useEventContext';

/**
 * Root application component.
 *
 * Sets up routing, loads auth state from storage, and renders the
 * navigation bar and page content.
 *
 * When accessed from an event subdomain (e.g. ctf2026.cyberorch.com),
 * the root "/" route renders the EventPage instead of the LandingPage.
 */
const App: React.FC = () => {
  const loadFromStorage = useAuthStore((s) => s.loadFromStorage);

  useEffect(() => {
    loadFromStorage();
  }, [loadFromStorage]);

  const isEventSubdomain = !!getEventSlug();

  return (
    <BrowserRouter>
      <div className="app">
        <Navbar />
        <main className="main-content">
          <Routes>
            {/* Public routes */}
            <Route
              path="/"
              element={isEventSubdomain ? <EventPage /> : <LandingPage />}
            />
            <Route path="/events" element={<EventPage />} />
            <Route path="/login" element={<LoginPage />} />
            <Route path="/register" element={<RegisterPage />} />

            {/* Protected routes */}
            <Route
              path="/dashboard"
              element={
                <ProtectedRoute>
                  <DashboardPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="/challenges"
              element={
                <ProtectedRoute>
                  <ChallengesPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="/scoreboard"
              element={
                <ProtectedRoute>
                  <ScoreboardPage />
                </ProtectedRoute>
              }
            />
            <Route
              path="/lab"
              element={
                <ProtectedRoute>
                  <LabPage />
                </ProtectedRoute>
              }
            />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  );
};

export default App;
