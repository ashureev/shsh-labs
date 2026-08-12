import { lazy, Suspense } from "react";
import { Routes, Route, Navigate, useNavigate, Link } from "react-router-dom";
import { Dashboard } from "./components/Dashboard";
import { ProvisioningState } from "./components/ProvisioningState";
import { useAuth } from "./context/useAuth";

const TerminalSession = lazy(() =>
  import("./components/TerminalSession").then((m) => ({
    default: m.TerminalSession,
  }))
);

const Navbar = () => {
  const { user } = useAuth();

  return (
    <header className="fixed top-0 left-0 right-0 z-50 border-b border-border bg-background-base/80 backdrop-blur-md">
      <div className="max-w-5xl mx-auto px-6 h-14 flex justify-between items-center">
        <Link
          to="/"
          className="flex items-center gap-2.5 group transition-opacity hover:opacity-90"
        >
          <div className="w-7 h-7 rounded-md bg-zinc-800 border border-zinc-700/80 flex items-center justify-center text-zinc-100 font-mono text-xs font-semibold shadow-sm">
            &gt;_
          </div>
          <div className="flex items-baseline gap-2">
            <span className="font-semibold text-sm text-text-primary tracking-tight">
              Playground
            </span>
            <span className="text-[11px] text-text-tertiary font-normal">
              Labs
            </span>
          </div>
        </Link>
        <div className="flex items-center gap-4 text-xs font-medium text-text-secondary">
          {user?.container_id ? (
            <Link
              to="/terminal"
              className="flex items-center gap-2 px-3 py-1.5 rounded-md bg-zinc-800 hover:bg-zinc-700 border border-zinc-700/80 text-text-primary transition-colors"
            >
              <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
              <span>Open Terminal</span>
            </Link>
          ) : (
            <div className="flex items-center gap-2 text-text-tertiary text-xs">
              <span className="w-1.5 h-1.5 rounded-full bg-zinc-600"></span>
              <span>Ready</span>
            </div>
          )}
        </div>
      </div>
    </header>
  );
};

export default function App() {
  const navigate = useNavigate();
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen bg-background-base flex flex-col items-center justify-center text-text-secondary">
        <div className="flex items-center gap-3">
          <div className="w-4 h-4 border-2 border-zinc-500 border-t-transparent rounded-full animate-spin"></div>
          <span className="text-xs text-text-secondary font-medium">Loading...</span>
        </div>
      </div>
    );
  }

  return (
    <Routes>
      <Route
        path="/"
        element={
          <div className="min-h-screen bg-background-base">
            <Navbar />
            <Dashboard
              onStartTerminal={() => {
                if (user?.container_id) {
                  navigate("/terminal");
                } else {
                  navigate("/provision");
                }
              }}
            />
          </div>
        }
      />
      <Route path="/dashboard" element={<Navigate to="/" replace />} />
      <Route
        path="/provision"
        element={<ProvisioningState onComplete={() => navigate("/terminal")} />}
      />
      <Route
        path="/terminal"
        element={
          <Suspense
            fallback={
              <div className="h-screen bg-background-base flex items-center justify-center text-text-secondary">
                <div className="flex items-center gap-3">
                  <div className="w-4 h-4 border-2 border-zinc-500 border-t-transparent rounded-full animate-spin"></div>
                  <span className="text-xs text-text-secondary">Connecting to terminal...</span>
                </div>
              </div>
            }
          >
            <TerminalSession
              onDestroy={() => {
                navigate("/");
              }}
            />
          </Suspense>
        }
      />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
