import React from 'react';
import { motion } from 'framer-motion';
import { ExclamationTriangleIcon, EyeIcon, EyeSlashIcon } from '@heroicons/react/24/outline';
import { cn } from '../../lib/utils';
import type { InputProps } from '../../types';

const inputVariants = {
  default: 'border-border-secondary focus:border-brand-500 focus:ring-brand-500/20',
  error: 'border-error-500 focus:border-error-500 focus:ring-error-500/20',
  success: 'border-success-500 focus:border-success-500 focus:ring-success-500/20',
};

const sizeVariants = {
  sm: 'px-3 py-2 text-sm',
  md: 'px-4 py-3 text-base',
  lg: 'px-5 py-4 text-lg',
};

export const Input: React.FC<InputProps> = ({
  label,
  error,
  success,
  helper,
  required,
  size = 'md',
  disabled,
  className,
  type = 'text',
  placeholder,
  icon,
  rightIcon,
  ...props
}) => {
  const [showPassword, setShowPassword] = React.useState(false);
  const [focused, setFocused] = React.useState(false);
  
  const inputType = type === 'password' && showPassword ? 'text' : type;
  const variant = error ? 'error' : success ? 'success' : 'default';
  
  const togglePasswordVisibility = () => {
    setShowPassword(!showPassword);
  };

  return (
    <div className="w-full">
      {/* Label */}
      {label && (
        <label className="block text-sm font-medium text-text-primary mb-2">
          {label}
          {required && <span className="text-error-500 ml-1">*</span>}
        </label>
      )}

      {/* Input Container */}
      <div className="relative">
        {/* Left Icon */}
        {icon && (
          <div className="absolute left-3 top-1/2 transform -translate-y-1/2 text-text-tertiary">
            {icon}
          </div>
        )}

        {/* Input Field */}
        <motion.div
          animate={{
            scale: focused ? 1.02 : 1,
          }}
          transition={{ duration: 0.1 }}
        >
          <input
            type={inputType}
            placeholder={placeholder}
            disabled={disabled}
            className={cn(
              'w-full rounded-lg border bg-surface transition-all duration-200',
              'focus:outline-none focus:ring-2',
              inputVariants[variant],
              sizeVariants[size],
              icon && 'pl-10',
              (rightIcon || type === 'password') && 'pr-10',
              disabled && 'opacity-50 cursor-not-allowed bg-surface-secondary',
              className
            )}
            onFocus={() => setFocused(true)}
            onBlur={() => setFocused(false)}
            {...props}
          />
        </motion.div>

        {/* Right Icon or Password Toggle */}
        {(rightIcon || type === 'password') && (
          <div className="absolute right-3 top-1/2 transform -translate-y-1/2">
            {type === 'password' ? (
              <motion.button
                type="button"
                onClick={togglePasswordVisibility}
                className="text-text-tertiary hover:text-text-primary transition-colors"
                whileHover={{ scale: 1.1 }}
                whileTap={{ scale: 0.9 }}
              >
                {showPassword ? (
                  <EyeSlashIcon className="w-5 h-5" />
                ) : (
                  <EyeIcon className="w-5 h-5" />
                )}
              </motion.button>
            ) : (
              <div className="text-text-tertiary">{rightIcon}</div>
            )}
          </div>
        )}
      </div>

      {/* Helper Text / Error / Success */}
      {(helper || error || success) && (
        <motion.div
          initial={{ opacity: 0, y: -5 }}
          animate={{ opacity: 1, y: 0 }}
          className="mt-2 flex items-start gap-2"
        >
          {error && (
            <>
              <ExclamationTriangleIcon className="w-4 h-4 text-error-500 mt-0.5 flex-shrink-0" />
              <span className="text-sm text-error-600">{error}</span>
            </>
          )}
          {success && !error && (
            <span className="text-sm text-success-600">{success}</span>
          )}
          {helper && !error && !success && (
            <span className="text-sm text-text-tertiary">{helper}</span>
          )}
        </motion.div>
      )}
    </div>
  );
};