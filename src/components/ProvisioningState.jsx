import { useState, useEffect, useRef, useCallback } from "react";
import { motion as Motion, AnimatePresence } from "framer-motion";
import CheckCircle2 from "lucide-react/dist/esm/icons/check-circle-2";
import AlertTriangle from "lucide-react/dist/esm/icons/alert-triangle";
import { useAuth } from "../context/AuthContext";

const ProvisioningHeaderLogs = () => (
    <div className="flex gap-4 text-[10px] tracking-[0.1em] uppercase text-text-secondary font-mono font-bold" aria-hidden="true">
        <span className="text-secondary-accent flex items-center gap-1.5"><CheckCircle2 size={10} /> Auth</span>
        <span className="text-primary-accent flex items-center gap-1.5">Provisioning</span>
        <span className="opacity-30">Terminal</span>
    </div>
);

// Global tracker to deduplicate requests across React StrictMode re-mounts in dev
let activeProvisionPromise = null;

export const ProvisioningState = ({ onComplete }) => {
    const { authFetch } = useAuth();
    const [logs, setLogs] = useState([]);
    const [progress, setProgress] = useState(0);
    const [retryCount, setRetryCount] = useState(0);
    const logEndRef = useRef(null);

    const addLog = useCallback((message, type = 'wait') => {
        const id = Math.random().toString(36).substring(7);
        setLogs(prev => [...prev, { id, message, type, time: new Date().toLocaleTimeString() }]);
    }, []);

    useEffect(() => {
        let isMounted = true;
        const controller = new AbortController();

        const startProvisioning = async () => {
            addLog("Verifying identity...", "info");
            setProgress(10);

            try {
                // Deduplicate: If another mount is already provisioning, share that promise
                if (!activeProvisionPromise) {
                    activeProvisionPromise = authFetch('/api/provision', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        // No signal here, so this survives the StrictMode re-mount
                    }).then(async res => {
                        const data = await res.json().catch(() => ({}));
                        if (!res.ok) throw { status: res.status, data };
                        return data;
                    }).finally(() => {
                        // Keep promise for a short duration to let StrictMode re-mounts sync
                        setTimeout(() => { activeProvisionPromise = null; }, 1000);
                    });
                } else {
                    addLog("Syncing with active compute lock...", "info");
                }

                const data = await activeProvisionPromise;

                if (!isMounted) return;

                addLog("Locking compute resources...", "info");
                setProgress(40);

                addLog("Attaching sandbox container...", "load");
                setProgress(70);

                addLog(`Instance ${data.container_id.substring(0, 8)} operational.`, "success");
                setProgress(100);

                setTimeout(() => {
                    if (isMounted) onComplete();
                }, 600);

            } catch (err) {
                if (err.name === 'AbortError') return;

                if (isMounted) {
                    if (err.status === 409) {
                        // Even with deduplication, if a real conflict occurs:
                        if (retryCount < 5) {
                            setTimeout(() => {
                                if (isMounted) setRetryCount(prev => prev + 1);
                            }, 1000);
                            return;
                        }
                    }
                    addLog("Security alert: " + (err.data?.error || err.message || "Provision failed"), "error");
                }
            }
        };

        startProvisioning();
        return () => {
            isMounted = false;
            controller.abort();
        };
    }, [onComplete, addLog, retryCount, authFetch]);

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
                        <span>Nominal</span>
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
                                    <span className={`flex-1 leading-relaxed ${log.type === 'error' ? 'text-red-400' : log.type === 'success' ? 'text-secondary-accent' : 'text-text-secondary'}`}>
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
                            <span className="text-[10px] uppercase tracking-tighter italic">Processing...</span>
                        </div>
                    )}
                </div>

                <div className="px-6 py-4 border-t border-border bg-background-surface">
                    <div className="h-1 bg-background-elevated w-full rounded-full overflow-hidden">
                        <Motion.div
                            className="h-full bg-primary-accent"
                            initial={{ width: 0 }}
                            animate={{ width: `${progress}%` }}
                            transition={{ type: "spring", bounce: 0, duration: 0.8 }}
                        />
                    </div>
                </div>
            </Motion.div>
        </div>
    );
};
