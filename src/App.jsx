import { lazy, Suspense } from "react";
import { Routes, Route, Navigate, useNavigate, Link } from "react-router-dom";
import { Dashboard } from "./components/Dashboard";
import { ProvisioningState } from "./components/ProvisioningState";
import { useAuth } from "./context/AuthContext";

const TerminalSession = lazy(() => import("./components/TerminalSession").then(m => ({ default: m.TerminalSession })));

const Navbar = () => {
  return (
    <nav className="fixed top-0 left-0 right-0 z-50 border-b border-border bg-background-base/90 backdrop-blur-md" role="navigation" aria-label="Main Navigation">
      <div className="max-w-7xl mx-auto px-6">
        <div className="flex justify-between items-center h-16">
          <div className="flex items-center gap-2">
            <Link to="/" className="text-primary-accent font-mono font-bold text-lg flex items-center tracking-tight hover:opacity-80 transition-opacity">
              {"> "} PLAYGROUND<span className="animate-pulse">_</span>
            </Link>
          </div>
          <div className="hidden md:flex items-center gap-8 text-[11px] uppercase tracking-widest text-text-secondary font-bold">
            <Link to="/" className="text-text-primary hover:text-primary-accent transition-colors">Dashboard</Link>
          </div>
        </div>
      </div>
    </nav>
  );
};

export default function App() {
  const navigate = useNavigate();
  const { loading } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen bg-background-base flex flex-col items-center justify-center font-mono text-text-secondary">
        <div className="flex items-center gap-3 animate-pulse">
          <div className="w-2 h-2 rounded-full bg-primary-accent"></div>
          <span className="text-xs tracking-widest uppercase italic font-bold">Initializing...</span>
        </div>
      </div>
    );
  }

  return (
    <Routes>
      <Route path="/" element={
        <div className="min-h-screen bg-background-base">
          <Navbar />
          <Dashboard onStartTerminal={() => navigate("/provision")} />
        </div>
      } />
      <Route path="/dashboard" element={<Navigate to="/" replace />} />
      <Route path="/provision" element={<ProvisioningState onComplete={() => navigate("/terminal")} />} />
      <Route path="/terminal" element={
        <Suspense fallback={<div className="h-screen bg-background-base flex items-center justify-center font-mono text-text-secondary animate-pulse text-[10px] uppercase font-bold tracking-widest">Attaching TTY Session...</div>}>
          <TerminalSession onDestroy={() => {
            navigate("/");
          }} />
        </Suspense>
      } />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
