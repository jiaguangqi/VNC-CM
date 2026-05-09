import React from "react";
import { Routes, Route, Navigate } from "react-router-dom";
import AppLayout from "./layouts/AppLayout";
import LoginPage from "./pages/LoginPage";
import DashboardPage from "./pages/DashboardPage";
import HostsPage from "./pages/HostsPage";
import DesktopsPage from "./pages/DesktopsPage";
import DesktopViewerPage from "./pages/DesktopViewerPage";
import SystemSettingsPage from "./pages/SystemSettingsPage";

const App: React.FC = () => {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<AppLayout />}>
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="hosts" element={<HostsPage />} />
        <Route path="desktops" element={<DesktopsPage />} />
        <Route path="desktops/:id" element={<DesktopViewerPage />} />
        <Route path="settings" element={<SystemSettingsPage />} />
      </Route>
    </Routes>
  );
};

export default App;
