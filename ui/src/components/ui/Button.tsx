import React from 'react';
import { motion } from 'framer-motion';
import { cn } from '../../lib/utils';
import type { ButtonProps } from '../../types';

const buttonVariants = {
  primary: 'bg-gradient-to-r from-brand-500 to-brand-600 text-white shadow-lg shadow-brand-500/30 hover:from-brand-400 hover:to-brand-500 hover:shadow-brand-500/40',
  secondary: 'bg-surface-secondary text-text-primary border border-border-secondary hover:bg-surface-tertiary hover:border-border-accent/50',
  danger: 'bg-gradient-to-r from-error-500 to-error-600 text-white shadow-lg shadow-error-500/30 hover:from-error-400 hover:to-error-500',
  ghost: 'text-text-secondary hover:text-text-primary hover:bg-surface-secondary/50',
  outline: 'border border-border-secondary text-text-primary hover:bg-surface-secondary hover:border-border-accent/50',
};

const sizeVariants = {
  sm: 'px-3 py-1.5 text-sm',
  md: 'px-4 py-2 text-base',
  lg: 'px-6 py-3 text-lg',
};

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ 
    className, 
    variant = 'primary', 
    size = 'md', 
    disabled = false, 
    loading = false, 
    icon, 
    children, 
    onClick,
    type = 'button',
    ...props 
  }, ref) => {
    return (
      <motion.button
        ref={ref}
        type={type}
        className={cn(
          // Base styles
          'inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-brand-500 focus:ring-offset-2 focus:ring-offset-background-secondary disabled:opacity-50 disabled:cursor-not-allowed disabled:transform-none',
          // Variant styles
          buttonVariants[variant],
          // Size styles
          sizeVariants[size],
          className
        )}
        disabled={disabled || loading}
        onClick={onClick}
        whileHover={disabled || loading ? {} : { scale: 1.02 }}
        whileTap={disabled || loading ? {} : { scale: 0.98 }}
        transition={{ type: "spring", stiffness: 400, damping: 17 }}
        {...props}
      >
        {loading && (
          <motion.div
            className="w-4 h-4 border-2 border-current border-t-transparent rounded-full"
            animate={{ rotate: 360 }}
            transition={{ duration: 1, repeat: Infinity, ease: "linear" }}
          />
        )}
        {icon && !loading && (
          <span className="flex-shrink-0">
            {icon}
          </span>
        )}
        {children}
      </motion.button>
    );
  }
);

Button.displayName = 'Button';