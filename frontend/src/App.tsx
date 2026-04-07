import { Navigate, Route, Routes } from 'react-router';

import { useAuth } from './context/AuthContext';
import { CampaignCreatePage } from './pages/CampaignCreatePage';
import { CampaignListPage } from './pages/CampaignListPage';
import { CampaignPlayPage } from './pages/CampaignPlayPage';
import { LoginPage } from './pages/LoginPage';
import { RegisterPage } from './pages/RegisterPage';

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-obsidian text-champagne">
        <p className="font-heading text-sm uppercase tracking-[0.2em] text-gold animate-pulse">
          Loading…
        </p>
      </main>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
}

function RedirectIfAuth({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) return null;
  if (isAuthenticated) return <Navigate to="/" replace />;

  return <>{children}</>;
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<RedirectIfAuth><LoginPage /></RedirectIfAuth>} />
      <Route path="/register" element={<RedirectIfAuth><RegisterPage /></RedirectIfAuth>} />
      <Route path="/" element={<RequireAuth><CampaignListPage /></RequireAuth>} />
      <Route path="/new" element={<RequireAuth><CampaignCreatePage /></RequireAuth>} />
      <Route path="/play/:id" element={<RequireAuth><CampaignPlayPage /></RequireAuth>} />
    </Routes>
  );
}

export default App;
