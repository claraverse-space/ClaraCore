import React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { 
  CheckCircleIcon, 
  XCircleIcon, 
  ExclamationTriangleIcon, 
  InformationCircleIcon,
  XMarkIcon 
} from '@heroicons/react/24/outline';
import { cn } from '../../lib/utils';

export interface ToastProps {
  id: string;
  type?: 'success' | 'error' | 'warning' | 'info';
  title: string;
  description?: string;
  duration?: number;
  closable?: boolean;
  onClose?: (id: string) => void;
}

const toastVariants = {
  success: {
    icon: CheckCircleIcon,
    iconColor: 'text-success-500',
    bgColor: 'bg-success-50 border-success-100 dark:bg-success-900/20 dark:border-success-800',
    titleColor: 'text-success-800 dark:text-success-200',
    descriptionColor: 'text-success-700 dark:text-success-300',
  },
  error: {
    icon: XCircleIcon,
    iconColor: 'text-error-500',
    bgColor: 'bg-error-50 border-error-100 dark:bg-error-900/20 dark:border-error-800',
    titleColor: 'text-error-800 dark:text-error-200',
    descriptionColor: 'text-error-700 dark:text-error-300',
  },
  warning: {
    icon: ExclamationTriangleIcon,
    iconColor: 'text-warning-500',
    bgColor: 'bg-warning-50 border-warning-100 dark:bg-warning-900/20 dark:border-warning-800',
    titleColor: 'text-warning-800 dark:text-warning-200',
    descriptionColor: 'text-warning-700 dark:text-warning-300',
  },
  info: {
    icon: InformationCircleIcon,
    iconColor: 'text-info-500',
    bgColor: 'bg-info-50 border-info-100 dark:bg-info-900/20 dark:border-info-800',
    titleColor: 'text-info-800 dark:text-info-200',
    descriptionColor: 'text-info-700 dark:text-info-300',
  },
};

export const Toast: React.FC<ToastProps> = ({
  id,
  type = 'info',
  title,
  description,
  duration = 5000,
  closable = true,
  onClose,
}) => {
  const variant = toastVariants[type];
  const IconComponent = variant.icon;

  React.useEffect(() => {
    if (duration > 0) {
      const timer = setTimeout(() => {
        onClose?.(id);
      }, duration);

      return () => clearTimeout(timer);
    }
  }, [duration, id, onClose]);

  return (
    <motion.div
      layout
      initial={{ opacity: 0, x: 300, scale: 0.8 }}
      animate={{ opacity: 1, x: 0, scale: 1 }}
      exit={{ opacity: 0, x: 300, scale: 0.8 }}
      transition={{ type: "spring", stiffness: 300, damping: 30 }}
      className={cn(
        'flex items-start gap-3 p-4 rounded-lg border shadow-lg backdrop-blur-sm max-w-md w-full',
        variant.bgColor
      )}
    >
      {/* Icon */}
      <IconComponent className={cn('w-5 h-5 flex-shrink-0 mt-0.5', variant.iconColor)} />

      {/* Content */}
      <div className="flex-1 min-w-0">
        <h4 className={cn('text-sm font-semibold', variant.titleColor)}>
          {title}
        </h4>
        {description && (
          <p className={cn('text-sm mt-1', variant.descriptionColor)}>
            {description}
          </p>
        )}
      </div>

      {/* Close Button */}
      {closable && onClose && (
        <motion.button
          className={cn('flex-shrink-0 p-1 rounded hover:bg-black/5 dark:hover:bg-white/5', variant.titleColor)}
          onClick={() => onClose(id)}
          whileHover={{ scale: 1.1 }}
          whileTap={{ scale: 0.9 }}
        >
          <XMarkIcon className="w-4 h-4" />
        </motion.button>
      )}
    </motion.div>
  );
};

export interface ToastContainerProps {
  toasts: ToastProps[];
  position?: 'top-right' | 'top-left' | 'bottom-right' | 'bottom-left' | 'top-center' | 'bottom-center';
  onRemove: (id: string) => void;
}

const positionClasses = {
  'top-right': 'top-4 right-4',
  'top-left': 'top-4 left-4',
  'bottom-right': 'bottom-4 right-4',
  'bottom-left': 'bottom-4 left-4',
  'top-center': 'top-4 left-1/2 transform -translate-x-1/2',
  'bottom-center': 'bottom-4 left-1/2 transform -translate-x-1/2',
};

export const ToastContainer: React.FC<ToastContainerProps> = ({
  toasts,
  position = 'top-right',
  onRemove,
}) => {
  return (
    <div className={cn('fixed z-50 flex flex-col gap-2', positionClasses[position])}>
      <AnimatePresence mode="popLayout">
        {toasts.map((toast) => (
          <Toast key={toast.id} {...toast} onClose={onRemove} />
        ))}
      </AnimatePresence>
    </div>
  );
};

// Toast Hook
export interface ToastState {
  toasts: ToastProps[];
  addToast: (toast: Omit<ToastProps, 'id'>) => void;
  removeToast: (id: string) => void;
  clearToasts: () => void;
}

export const useToast = (): ToastState => {
  const [toasts, setToasts] = React.useState<ToastProps[]>([]);

  const addToast = React.useCallback((toast: Omit<ToastProps, 'id'>) => {
    const id = Math.random().toString(36).substr(2, 9);
    const newToast = { ...toast, id };
    setToasts(prev => [...prev, newToast]);
  }, []);

  const removeToast = React.useCallback((id: string) => {
    setToasts(prev => prev.filter(toast => toast.id !== id));
  }, []);

  const clearToasts = React.useCallback(() => {
    setToasts([]);
  }, []);

  return { toasts, addToast, removeToast, clearToasts };
};