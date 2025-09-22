import { RiCpuLine } from "react-icons/ri";
import { NavLink, type NavLinkRenderProps } from "react-router-dom";
import { useTheme } from "../contexts/ThemeProvider";
import ConnectionStatusIcon from "./ConnectionStatus";

export function Header() {
  const { screenWidth } = useTheme();

  const navLinkClass = ({ isActive }: NavLinkRenderProps) =>
    `px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 ${
      isActive
        ? "bg-primary text-white shadow-sm"
        : "text-gray-300 hover:text-white hover:bg-surface-hover"
    }`;

  return (
    <nav className="flex items-center justify-between glass-surface px-6 py-3 h-16 sticky top-0 z-50">
      {/* ClaraCore Branding */}
      <div className="flex items-center gap-3">
        <div className="flex items-center justify-center w-8 h-8 bg-gradient-to-br from-primary to-sakura-600 rounded-lg shadow-sm">
          <RiCpuLine className="w-5 h-5 text-white" />
        </div>
        <div className="flex flex-col">
          <h1 className="text-lg font-bold text-white leading-none">
            ClaraCore
          </h1>
          {screenWidth !== "xs" && (
            <span className="text-xs text-gray-400 leading-none">
              AI Model Manager
            </span>
          )}
        </div>
      </div>

      {/* Navigation & Status */}
      <div className="flex items-center gap-2">
        {/* Navigation Links */}
        <div className="flex items-center gap-1 mr-4">
          <NavLink to="/" className={navLinkClass}>
            Logs
          </NavLink>
          <NavLink to="/models" className={navLinkClass}>
            Models
          </NavLink>
          <NavLink to="/activity" className={navLinkClass}>
            Activity
          </NavLink>
        </div>

        {/* Status */}
        <div className="flex items-center gap-2 pl-4 border-l border-gray-600">
          <ConnectionStatusIcon />
        </div>
      </div>
    </nav>
  );
}
