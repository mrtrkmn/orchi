import React from 'react';

/**
 * Lab access page — manage team lab environment.
 *
 * Features:
 * - Lab status overview (running/stopped/expired)
 * - Exercise instance list with status and IP
 * - Reset buttons for lab and individual exercises
 * - VPN configuration download
 * - Browser-based desktop access (Guacamole iframe)
 */
export const LabPage: React.FC = () => {
  return (
    <div className="lab-page">
      <div className="page-header">
        <h1>
          <span className="accent">{'>'}</span> Lab Environment
        </h1>
      </div>

      <div className="lab-grid">
        {/* Lab Status */}
        <div className="card">
          <h3>Lab Status</h3>
          <div className="lab-status">
            <span className="status-badge status-inactive">Not Created</span>
            <p className="text-muted">
              Join an event and create a team to provision your lab environment.
            </p>
          </div>
        </div>

        {/* VPN Access */}
        <div className="card">
          <h3>VPN Access</h3>
          <p className="text-muted">
            Download your WireGuard VPN configuration to access lab resources
            directly from your machine.
          </p>
          <button className="btn btn-secondary" disabled>
            Download VPN Config
          </button>
          <div className="vpn-instructions">
            <h4>Connection Instructions</h4>
            <ol>
              <li>Install WireGuard from <a href="https://www.wireguard.com/install/" target="_blank" rel="noopener noreferrer">wireguard.com</a></li>
              <li>Import the downloaded configuration file</li>
              <li>Activate the tunnel</li>
              <li>Verify connection with <code>wg show</code></li>
            </ol>
          </div>
        </div>

        {/* Exercises */}
        <div className="card card-full">
          <h3>Exercises</h3>
          <p className="text-muted">No exercises running. Start a lab to see your exercise instances.</p>
        </div>

        {/* Browser Access */}
        <div className="card card-full">
          <h3>Browser Desktop</h3>
          <p className="text-muted">
            Browser-based desktop access will be available once your lab is running.
            Access your Kali Linux VM directly from the browser.
          </p>
        </div>
      </div>
    </div>
  );
};
