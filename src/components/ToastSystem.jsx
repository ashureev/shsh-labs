import { useState, useEffect, useCallback } from 'react';

const TOAST_DURATION = 5000;
let toastCounter = 0;

function ToastItem({ toast, onDismiss }) {
  useEffect(() => {
    const timer = setTimeout(() => onDismiss(toast.id), TOAST_DURATION);
    return () => clearTimeout(timer);
  }, [toast.id, onDismiss]);

  const getBorderColor = () => {
    switch (toast.type) {
      case 'error':
        return 'border-red-500/40 text-red-400';
      case 'success':
        return 'border-emerald-500/40 text-emerald-400';
      case 'warning':
      case 'safety-tier2':
        return 'border-amber-500/40 text-amber-400';
      default:
        return 'border-zinc-700 text-sky-400';
    }
  };

  return (
    <div
      className={`flex items-start gap-3 p-3.5 rounded-xl border bg-zinc-900/95 backdrop-blur-md shadow-2xl transition-all duration-200 ${getBorderColor()}`}
    >
      <div className="flex-1 min-w-0">
        {toast.title && (
          <p className="text-xs font-semibold text-text-primary mb-0.5">
            {toast.title}
          </p>
        )}
        <p className="text-xs text-text-secondary leading-relaxed">
          {toast.message}
        </p>
      </div>
      <button
        onClick={() => onDismiss(toast.id)}
        className="text-zinc-500 hover:text-zinc-300 p-0.5 rounded transition-colors"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <line x1="18" y1="6" x2="6" y2="18" />
          <line x1="6" y1="6" x2="18" y2="18" />
        </svg>
      </button>
    </div>
  );
}

export function ToastContainer({ toasts, onDismiss }) {
  if (toasts.length === 0) return null;

  return (
    <div className="fixed top-4 right-4 z-[100] flex flex-col gap-2 max-w-sm w-full pointer-events-none">
      {toasts.map((toast) => (
        <div key={toast.id} className="pointer-events-auto">
          <ToastItem toast={toast} onDismiss={onDismiss} />
        </div>
      ))}
    </div>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useToast() {
  const [toasts, setToasts] = useState([]);

  const addToast = useCallback((toast) => {
    toastCounter += 1;
    const id = `${Date.now()}-${toastCounter}`;
    setToasts((prev) => [...prev, { ...toast, id }]);
    return id;
  }, []);

  const dismissToast = useCallback((id) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const dismissAll = useCallback(() => {
    setToasts([]);
  }, []);

  return { toasts, addToast, dismissToast, dismissAll };
}
