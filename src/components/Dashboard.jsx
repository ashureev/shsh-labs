import { useState, useEffect, memo } from "react";
import { motion as Motion } from "framer-motion";
import { useAuth } from "../context/useAuth";

export const Dashboard = memo(({ onStartTerminal }) => {
  const { user, checkAuth } = useAuth();
  const [isLaunching, setIsLaunching] = useState(false);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  const handleLaunch = async () => {
    if (isLaunching) return;
    setIsLaunching(true);
    try {
      await onStartTerminal();
    } finally {
      setIsLaunching(false);
    }
  };

  const isRunning = Boolean(user?.container_id);

  return (
    <div className="min-h-screen bg-background-base text-text-primary flex flex-col justify-center items-center px-6 pt-14 pb-12">
      <div className="w-full max-w-lg">
        {/* Header Title */}
        <Motion.div
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25 }}
          className="text-center mb-8"
        >
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-zinc-900 border border-zinc-800 text-xs text-text-secondary mb-4">
            <span
              className={`w-2 h-2 rounded-full ${
                isRunning ? "bg-emerald-400" : "bg-zinc-500"
              }`}
            />
            <span>{isRunning ? "Active Sandbox" : "Ready to Launch"}</span>
          </div>

          <h1 className="text-2xl sm:text-3xl font-semibold tracking-tight text-text-primary mb-2">
            Linux Playground
          </h1>
          <p className="text-text-secondary text-sm leading-relaxed max-w-sm mx-auto">
            Instant disposable Ubuntu environment with terminal access and built-in AI assistance.
          </p>
        </Motion.div>

        {/* Action Card */}
        <Motion.div
          initial={{ opacity: 0, y: 14 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3, delay: 0.05 }}
          className="bg-background-surface border border-border rounded-xl p-6 shadow-xl relative"
        >
          <div className="flex flex-col gap-4">
            <div className="grid grid-cols-2 gap-3 text-xs">
              <div className="p-3.5 rounded-lg bg-zinc-900/70 border border-zinc-800/80">
                <span className="text-text-tertiary block text-[11px] font-medium mb-1">
                  Environment
                </span>
                <span className="text-text-primary font-medium font-mono">
                  Ubuntu 22.04 LTS
                </span>
              </div>
              <div className="p-3.5 rounded-lg bg-zinc-900/70 border border-zinc-800/80">
                <span className="text-text-tertiary block text-[11px] font-medium mb-1">
                  Container State
                </span>
                <span
                  className={`font-medium ${
                    isRunning ? "text-emerald-400" : "text-text-secondary"
                  }`}
                >
                  {isRunning ? "Running" : "Standby"}
                </span>
              </div>
            </div>

            <button
              onClick={handleLaunch}
              disabled={isLaunching}
              className="w-full py-3 px-4 rounded-lg bg-zinc-100 hover:bg-white text-zinc-900 font-medium text-sm transition-all flex items-center justify-center gap-2 shadow-sm active:scale-[0.99] disabled:opacity-60 cursor-pointer"
            >
              {isLaunching ? (
                <>
                  <div className="w-4 h-4 border-2 border-zinc-800 border-t-transparent rounded-full animate-spin" />
                  <span>Connecting...</span>
                </>
              ) : (
                <>
                  <svg
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <polyline points="4 17 10 11 4 5"></polyline>
                    <line x1="12" y1="19" x2="20" y2="19"></line>
                  </svg>
                  <span>{isRunning ? "Resume Terminal" : "Launch Terminal"}</span>
                </>
              )}
            </button>
          </div>
        </Motion.div>
      </div>
    </div>
  );
});
