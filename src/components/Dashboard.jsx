import React, { memo } from "react";
import { motion as Motion } from "framer-motion";
import Terminal from "lucide-react/dist/esm/icons/terminal";
import Activity from "lucide-react/dist/esm/icons/activity";
import { useAuth } from "../context/AuthContext";

const StatCard = memo(({ label, value, icon }) => {
    const IconComponent = icon;
    return (
    <div className="group p-6 rounded-lg border border-border-subtle hover:border-primary-accent/30 transition-all duration-300 bg-background-surface/50 hover:bg-background-elevated">
        <div className="flex justify-between items-start mb-4">
            <span className="text-text-secondary text-[10px] uppercase tracking-widest font-bold font-mono">{label}</span>
            <IconComponent size={14} className="text-text-tertiary group-hover:text-primary-accent transition-colors" />
        </div>
        <div className="text-xl font-mono text-text-primary tracking-tight">{value}</div>
    </div>
    );
});

const QuickAction = memo(({ title, description, icon, onClick, primary = false }) => {
    const IconComponent = icon;
    return (
    <button
        onClick={onClick}
        aria-label={title}
        className={`w-full text-left p-5 rounded-lg border transition-all duration-200 group flex items-start gap-4 focus-ring ${primary
            ? "bg-background-elevated border-border hover:border-primary-accent/30 active:scale-[0.99]"
            : "bg-transparent border-transparent hover:bg-background-surface/50 text-text-secondary hover:text-text-primary active:scale-[0.99]"
            }`}
    >
        <div className={`mt-0.5 p-2 rounded-md transition-colors ${primary ? "bg-primary-accent/10 text-primary-accent" : "text-text-tertiary group-hover:text-text-secondary"}`}>
            <IconComponent size={18} />
        </div>
        <div>
            <h3 className={`font-medium mb-1 ${primary ? "text-text-primary" : "text-text-secondary group-hover:text-text-primary"}`}>{title}</h3>
            <p className="text-xs text-text-secondary/70 leading-relaxed">{description}</p>
        </div>
    </button>
    );
});

export const Dashboard = ({ onStartTerminal }) => {
    const { user } = useAuth();
    const [isLaunching, setIsLaunching] = React.useState(false);

    const handleLaunch = async () => {
        if (isLaunching) return;
        setIsLaunching(true);
        try {
            await onStartTerminal();
        } finally {
            setIsLaunching(false);
        }
    };

    return (
        <div className="min-h-screen bg-background-base text-text-primary selection:bg-primary-accent/20">
            <div className="max-w-5xl mx-auto pt-24 px-6 pb-12">
                <header className="mb-20">
                    <Motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }}>
                        <h1 className="text-2xl font-light text-text-primary mb-3">
                            Welcome, <span className="font-medium text-white">{user?.username || "Developer"}</span>
                        </h1>
                        <p className="text-text-secondary text-sm max-w-md leading-relaxed">
                            Your isolated compute stack is ready for provisioning.
                        </p>
                    </Motion.div>
                </header>

                <div className="grid grid-cols-2 gap-4 mb-16 max-w-md">
                    <StatCard label="Active Status" value={user?.container_id ? "Running" : "Standby"} icon={Activity} />
                    <StatCard label="Environment" value="Ubuntu 22.04 LTS" icon={Terminal} />
                </div>

                <div className="max-w-2xl">
                    <section aria-label="Quick Actions">
                        <h2 className="text-[10px] font-bold text-text-tertiary uppercase tracking-[0.2em] mb-6">Launch Protocol</h2>
                        <div className="grid gap-3">
                            <QuickAction
                                title={isLaunching ? "Provisioning..." : "Open Terminal Session"}
                                description="Start or resume your isolated shell environment."
                                icon={Terminal}
                                onClick={handleLaunch}
                                primary={true}
                            />
                        </div>
                    </section>
                </div>
            </div>
        </div>
    );
};
