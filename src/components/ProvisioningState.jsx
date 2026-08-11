import { useState, useEffect, useRef, useCallback } from "react";
import { motion as Motion, AnimatePresence } from "framer-motion";
import CheckCircle2 from "lucide-react/dist/esm/icons/check-circle-2";
import AlertTriangle from "lucide-react/dist/esm/icons/alert-triangle";
import { useAuth } from "../context/useAuth";

const ProvisioningHeaderLogs = () => (
  <div className="flex gap-4 text-[10px] tracking-[0.1em] uppercase text-text-secondary font-mono font-bold" aria-hidden="true">
    <span className="text-secondary-accent flex items-center gap-1.5"><CheckCircle2 size={10} /> Auth</span>
    <span className="text-primary-accent flex items-center gap-1.5">Provisioning</span>
    <span className="opacity-30">Terminal</span>
  </div>
);

export const ProvisioningState = ({ onComplete }) => {
  const { authFetch, checkAuth } = useAuth();
  const [logs, setLogs] = useState([]);
  const [progress, setProgress] = useState(20);
  const logEndRef = useRef(null);
  const provisioningRef = useRef(false);

  const addLog = useCallback((message, type = 'wait') => {
    const id = Math.random().toString(36).substring(7);
    setLogs((prev) => [...prev, { id, message, type, time: new Date().toLocaleTimeString() }]);
  }, []);

  useEffect(() => {
    if (provisioningRef.current) return;
    provisioningRef.current = true;
    let isMounted = true;

    const startProvisioning = async () => {
      addLog("Verifying identity & compute stack...", "info");
      setProgress(40);

      try {
        const res = await authFetch('/api/provision', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
        });

        const data = await res.json().catch(() => ({}));
        if (!res.ok) throw new Error(data.error || 'Provision failed');

        if (!isMounted) return;

        addLog(`Sandbox container ${data.container_id ? data.container_id.substring(0, 8) : 'active'} ready.`, "success");
        setProgress(100);
        await checkAuth();

        // Immediate transition to terminal
        if (isMounted) {
          onComplete();
        }
      } catch (err) {
        if (isMounted) {
          addLog("Provisioning notice: " + (err.message || "Failed to attach"), "error");
        }
      }
    };

    startProvisioning();

    return () => {
      isMounted = false;
    };
  }, [addLog, authFetch, checkAuth, onComplete]);

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  return (
    <div className="min-h-screen bg-background-base flex items-center justify-center p-6 font-mono selection:bg-primary-accent/20">
      <Motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        className="w-full max-w-lg bg-background-surface border border-border rounded-lg shadow-2xl overflow-hidden"
        role="status"
        aria-live="polite"
      >
        <div className="bg-background-elevated px-6 py-4 border-b border-border flex justify-between items-center">
          <ProvisioningHeaderLogs />
          <div className="flex items-center gap-2 text-[10px] text-text-tertiary uppercase tracking-widest font-bold">
            <div className="w-1.5 h-1.5 rounded-full bg-secondary-accent animate-pulse"></div>
            <span>Active</span>
          </div>
        </div>

        <div className="p-6 h-72 overflow-y-auto bg-background-base/50 scrollbar-hide flex flex-col">
          <div className="space-y-4 flex-1">
            <AnimatePresence mode="popLayout">
              {logs.map((log) => (
                <Motion.div
                  key={log.id}
                  initial={{ opacity: 0, x: -8 }}
                  animate={{ opacity: 1, x: 0 }}
                  className="flex gap-4 text-xs font-mono"
                >
                  <span className="flex-shrink-0 text-[9px] w-14 text-right opacity-30 mt-1">
                    [{log.time}]
                  </span>
                  <span
                    className={`flex-1 leading-relaxed ${
                      log.type === 'error'
                        ? 'text-red-400'
                        : log.type === 'success'
                        ? 'text-secondary-accent font-bold'
                        : 'text-text-secondary'
                    }`}
                  >
                    {log.type === 'error' && <AlertTriangle size={10} className="inline mr-2" />}
                    {log.message}
                  </span>
                </Motion.div>
              ))}
            </AnimatePresence>
            <div ref={logEndRef} />
          </div>
          {progress < 100 && (
            <div className="mt-4 flex items-center gap-2 opacity-50">
              <div className="w-1.5 h-3 bg-primary-accent animate-pulse"></div>
              <span className="text-[10px] uppercase tracking-tighter italic">Attaching...</span>
            </div>
          )}
        </div>

        <div className="px-6 py-4 border-t border-border bg-background-surface">
          <div className="h-1 bg-background-elevated w-full rounded-full overflow-hidden">
            <Motion.div
              className="h-full bg-primary-accent"
              initial={{ width: 0 }}
              animate={{ width: `${progress}%` }}
              transition={{ type: "spring", bounce: 0, duration: 0.3 }}
            />
          </div>
        </div>
      </Motion.div>
    </div>
  );
};
