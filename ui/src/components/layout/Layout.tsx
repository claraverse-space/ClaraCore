import React from 'react';
import { motion } from 'framer-motion';
import { cn } from '../../lib/utils';

export interface LayoutProps {
  children: React.ReactNode;
  className?: string;
}

export const Layout: React.FC<LayoutProps> = ({ children, className }) => {
  return (
    <div className={cn('min-h-screen bg-background text-text-primary', className)}>
      {children}
    </div>
  );
};

export interface HeaderProps {
  children: React.ReactNode;
  className?: string;
  sticky?: boolean;
}

export const Header: React.FC<HeaderProps> = ({ children, className, sticky = true }) => {
  return (
    <motion.header
      className={cn(
        'bg-surface border-b border-border-secondary backdrop-blur-sm z-40',
        sticky && 'sticky top-0',
        className
      )}
      initial={{ y: -100 }}
      animate={{ y: 0 }}
      transition={{ type: "spring", stiffness: 300, damping: 30 }}
    >
      <div className="container mx-auto px-4 py-4">
        {children}
      </div>
    </motion.header>
  );
};

export interface SidebarProps {
  children: React.ReactNode;
  className?: string;
  width?: 'sm' | 'md' | 'lg';
  position?: 'left' | 'right';
}

const sidebarWidths = {
  sm: 'w-64',
  md: 'w-72',
  lg: 'w-80',
};

export const Sidebar: React.FC<SidebarProps> = ({ 
  children, 
  className, 
  width = 'md',
  position = 'left'
}) => {
  return (
    <motion.aside
      className={cn(
        'bg-surface border-border-secondary h-full overflow-y-auto',
        sidebarWidths[width],
        position === 'left' ? 'border-r' : 'border-l',
        className
      )}
      initial={{ x: position === 'left' ? -300 : 300 }}
      animate={{ x: 0 }}
      transition={{ type: "spring", stiffness: 300, damping: 30 }}
    >
      <div className="p-6">
        {children}
      </div>
    </motion.aside>
  );
};

export interface MainContentProps {
  children: React.ReactNode;
  className?: string;
  padding?: 'none' | 'sm' | 'md' | 'lg';
}

const paddingVariants = {
  none: 'p-0',
  sm: 'p-4',
  md: 'p-6',
  lg: 'p-8',
};

export const MainContent: React.FC<MainContentProps> = ({ 
  children, 
  className, 
  padding = 'md' 
}) => {
  return (
    <motion.main
      className={cn('flex-1 overflow-y-auto', paddingVariants[padding], className)}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.3 }}
    >
      <div className="container mx-auto">
        {children}
      </div>
    </motion.main>
  );
};

export interface FooterProps {
  children: React.ReactNode;
  className?: string;
}

export const Footer: React.FC<FooterProps> = ({ children, className }) => {
  return (
    <motion.footer
      className={cn('bg-surface border-t border-border-secondary mt-auto', className)}
      initial={{ y: 100 }}
      animate={{ y: 0 }}
      transition={{ type: "spring", stiffness: 300, damping: 30 }}
    >
      <div className="container mx-auto px-4 py-6">
        {children}
      </div>
    </motion.footer>
  );
};

// Compound layout components
export interface DashboardLayoutProps {
  children: React.ReactNode;
  header?: React.ReactNode;
  sidebar?: React.ReactNode;
  footer?: React.ReactNode;
  sidebarWidth?: 'sm' | 'md' | 'lg';
  className?: string;
}

export const DashboardLayout: React.FC<DashboardLayoutProps> = ({
  children,
  header,
  sidebar,
  footer,
  sidebarWidth = 'md',
  className,
}) => {
  return (
    <Layout className={cn('flex flex-col', className)}>
      {header && <Header>{header}</Header>}
      
      <div className="flex flex-1 overflow-hidden">
        {sidebar && (
          <Sidebar width={sidebarWidth}>
            {sidebar}
          </Sidebar>
        )}
        
        <MainContent>
          {children}
        </MainContent>
      </div>
      
      {footer && <Footer>{footer}</Footer>}
    </Layout>
  );
};