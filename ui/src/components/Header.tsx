import { CpuIcon } from "lucide-react";
import { NavLink, type NavLinkRenderProps } from "react-router-dom";
import { useTheme } from "../contexts/ThemeProvider";
import ConnectionStatusIcon from "./ConnectionStatus";
import { motion } from "framer-motion";

export function Header() {
  const { screenWidth } = useTheme();

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
        <div className="flex items-center justify-center w-10 h-10 bg-gradient-to-br from-brand-500 to-brand-600 rounded-xl shadow-lg">
          <CpuIcon className="w-5 h-5 text-white" />
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

        {/* Status */}
        <div className="flex items-center gap-2 pl-4 border-l border-border-secondary">
          <ConnectionStatusIcon />
        </div>
      </div>
    </motion.nav>
  );
}
