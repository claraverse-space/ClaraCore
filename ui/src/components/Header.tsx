import { RotateCcw, ChevronDown } from "lucide-react";
import { NavLink, type NavLinkRenderProps } from "react-router-dom";
import { useTheme } from "../contexts/ThemeProvider";
import ConnectionStatusIcon from "./ConnectionStatus";
import { motion, AnimatePresence } from "framer-motion";
import { useState, useRef, useEffect } from "react";
import logoSrc from "../assets/logo.svg";

export function Header() {
  const { screenWidth } = useTheme();
  const [isRestarting, setIsRestarting] = useState(false);
  const [showRestartMenu, setShowRestartMenu] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setShowRestartMenu(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleSoftRestart = async () => {
    if (isRestarting) return;
    
    try {
      setIsRestarting(true);
      setShowRestartMenu(false);
      const response = await fetch('/api/server/restart', {
        method: 'POST',
      });
      
      if (response.ok) {
        // Soft restart doesn't kill the server, just reload after a moment
        setTimeout(() => {
          setIsRestarting(false);
          window.location.reload();
        }, 3000);
      }
    } catch (error) {
      console.error('Failed to soft restart server:', error);
      setIsRestarting(false);
    }
  };

  const handleHardRestart = async () => {
    if (isRestarting) return;
    
    try {
      setIsRestarting(true);
      setShowRestartMenu(false);
      const response = await fetch('/api/server/restart/hard', {
        method: 'POST',
      });
      
      if (response.ok) {
        // Hard restart kills and respawns the server
        setTimeout(() => {
          window.location.reload();
        }, 2000);
      }
    } catch (error) {
      console.error('Failed to hard restart server:', error);
      setIsRestarting(false);
    }
  };

  const navLinkClass = ({ isActive }: NavLinkRenderProps) =>
    `inline-flex items-center px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 ${
      isActive
        ? "bg-brand-500 text-white shadow-sm"
        : "text-text-secondary hover:text-text-primary hover:bg-surface-secondary"
    }`;

  const secondaryNavLinkClass = ({ isActive }: NavLinkRenderProps) =>
    `inline-flex items-center px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 ${
      isActive
        ? "bg-surface-secondary text-text-primary border border-border-accent"
        : "text-text-tertiary hover:text-text-secondary hover:bg-surface-secondary/50"
    }`;

  return (
    <motion.nav 
      initial={{ y: -100, opacity: 0 }}
      animate={{ y: 0, opacity: 1 }}
      className="flex items-center justify-between bg-surface/80 backdrop-blur-md border-b border-border-secondary px-6 py-3 h-16 sticky top-0 z-50"
    >
      {/* ClaraCore Branding */}
      <motion.div 
        className="flex items-center gap-3"
        whileHover={{ scale: 1.02 }}
      >
        <div className="flex items-center justify-center w-10 h-10 rounded-xl shadow-lg overflow-hidden">
          <img 
            src={logoSrc} 
            alt="ClaraCore Logo" 
            className="w-full h-full object-contain"
          />
        </div>
        <div className="flex flex-col">
          <h1 className="text-xl font-bold text-text-primary leading-none">
            ClaraCore
          </h1>
          {screenWidth !== "xs" && (
            <span className="text-xs text-text-tertiary leading-none">
              AI Model Manager
            </span>
          )}
        </div>
      </motion.div>

      {/* Navigation & Status */}
      <div className="flex items-center gap-4">
        {/* Navigation Links */}
        <nav className="flex items-center gap-1">
          {/* Primary Workflow - Core Features */}
          <NavLink to="/setup" className={navLinkClass}>
            Setup
          </NavLink>
          <NavLink to="/models" className={navLinkClass}>
            Models
          </NavLink>
          <NavLink to="/config" className={navLinkClass}>
            Configuration
          </NavLink>
          <NavLink to="/activity" className={navLinkClass}>
            Activity
          </NavLink>
          
          {/* Separator */}
          <div className="w-px h-6 bg-border-secondary mx-2"></div>
          
          {/* Secondary Tools */}
          <NavLink to="/downloader" className={secondaryNavLinkClass}>
            Downloader
          </NavLink>
          <NavLink to="/" className={secondaryNavLinkClass}>
            Logs
          </NavLink>
        </nav>

        {/* Status & Actions */}
        <div className="flex items-center gap-3 pl-4 border-l border-border-secondary">
          <div className="relative" ref={dropdownRef}>
            <motion.button
              onClick={() => setShowRestartMenu(!showRestartMenu)}
              disabled={isRestarting}
              className={`inline-flex items-center gap-2 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 ${
                isRestarting
                  ? "bg-amber-500/20 text-amber-600 cursor-not-allowed"
                  : "text-text-tertiary hover:text-text-secondary hover:bg-surface-secondary/50"
              }`}
              whileHover={!isRestarting ? { scale: 1.02 } : {}}
              whileTap={!isRestarting ? { scale: 0.98 } : {}}
              title="Restart Server Options"
            >
              <RotateCcw 
                className={`w-4 h-4 ${isRestarting ? 'animate-spin' : ''}`} 
              />
              {screenWidth !== "xs" && (
                <span>{isRestarting ? "Restarting..." : "Restart"}</span>
              )}
              {!isRestarting && (
                <ChevronDown className="w-3 h-3" />
              )}
            </motion.button>

            {/* Restart Options Dropdown */}
            <AnimatePresence>
              {showRestartMenu && !isRestarting && (
                <motion.div
                  initial={{ opacity: 0, y: -10, scale: 0.95 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: -10, scale: 0.95 }}
                  className="absolute top-full right-0 mt-2 w-64 bg-gray-900/95 backdrop-blur-md border border-gray-700 rounded-lg shadow-2xl z-50"
                >
                  <div className="p-2">
                    <button
                      onClick={handleSoftRestart}
                      className="w-full text-left px-3 py-2 rounded-md hover:bg-gray-800/80 transition-colors group"
                    >
                      <div className="flex items-center gap-3">
                        <RotateCcw className="w-4 h-4 text-green-400" />
                        <div>
                          <div className="font-medium text-white">Soft Restart</div>
                          <div className="text-xs text-gray-400">Reload config & restart models</div>
                        </div>
                      </div>
                    </button>
                    <button
                      onClick={handleHardRestart}
                      className="w-full text-left px-3 py-2 rounded-md hover:bg-gray-800/80 transition-colors group"
                    >
                      <div className="flex items-center gap-3">
                        <RotateCcw className="w-4 h-4 text-orange-400" />
                        <div>
                          <div className="font-medium text-white">Hard Restart</div>
                          <div className="text-xs text-gray-400">Restart entire server process</div>
                        </div>
                      </div>
                    </button>
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
          <ConnectionStatusIcon />
        </div>
      </div>
    </motion.nav>
  );
}
