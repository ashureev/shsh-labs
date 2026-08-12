import { useState, useEffect, useRef } from "react";
import { motion as Motion } from "framer-motion";
import { useAuth } from "../context/useAuth";

export const ProvisioningState = ({ onComplete }) => {
  const { authFetch, checkAuth } = useAuth();
  const [statusMessage, setStatusMessage] = useState("Preparing container environment...");
  const [errorMessage, setErrorMessage] = useState(null);
  const [progress, setProgress] = useState(30);
  const provisioningRef = useRef(false);

  useEffect(() => {
    if (provisioningRef.current) return;
    provisioningRef.current = true;
    let isMounted = true;

    const startProvisioning = async () => {
      setStatusMessage("Starting Linux container...");
      setProgress(60);

      try {
        const res = await authFetch("/api/provision", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
        });

        const data = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(data.error || "Failed to start environment");

        if (!isMounted) return;

        setStatusMessage("Environment ready. Opening terminal...");
        setProgress(100);
        await checkAuth();

        if (isMounted) {
          onComplete();
        }
      } catch (err) {
        if (isMounted) {
          setErrorMessage(err.message || "Failed to initialize environment");
        }
      }
    };

    startProvisioning();

    return () => {
      isMounted = false;
    };
  }, [authFetch, checkAuth, onComplete]);

  return (
    <div className="min-h-screen bg-background-base flex items-center justify-center p-6">
      <Motion.div
        initial={{ opacity: 0, scale: 0.98 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.2 }}
        className="w-full max-w-sm bg-background-surface border border-border rounded-xl p-6 shadow-2xl"
        role="status"
        aria-live="polite"
      >
        <div className="flex items-center gap-3.5 mb-5">
          {errorMessage ? (
            <div className="w-10 h-10 rounded-lg bg-red-500/10 border border-red-500/20 flex items-center justify-center text-red-400 shrink-0">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="12" />
                <line x1="12" y1="16" x2="12.01" y2="16" />
              </svg>
            </div>
          ) : (
            <div className="w-10 h-10 rounded-lg bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-200 shrink-0">
              <div className="w-4 h-4 border-2 border-zinc-400 border-t-transparent rounded-full animate-spin" />
            </div>
          )}
          <div className="min-w-0">
            <h2 className="text-sm font-semibold text-text-primary">
              {errorMessage ? "Setup Failed" : "Starting Playground"}
            </h2>
            <p className="text-xs text-text-secondary truncate mt-0.5">
              {errorMessage || statusMessage}
            </p>
          </div>
        </div>

        {!errorMessage && (
          <div className="h-1.5 bg-zinc-900 rounded-full overflow-hidden w-full border border-zinc-800">
            <Motion.div
              className="h-full bg-zinc-200 rounded-full"
              initial={{ width: 0 }}
              animate={{ width: `${progress}%` }}
              transition={{ duration: 0.3, ease: "easeOut" }}
            />
          </div>
        )}

        {errorMessage && (
          <div className="mt-4 pt-4 border-t border-border flex justify-end">
            <button
              onClick={() => window.location.reload()}
              className="px-3.5 py-1.5 bg-zinc-800 hover:bg-zinc-700 text-xs font-medium text-zinc-200 rounded-md transition-colors"
            >
              Try Again
            </button>
          </div>
        )}
      </Motion.div>
    </div>
  );
};
