import React from 'react';
import { motion } from 'framer-motion';
import { ChevronUpIcon, ChevronDownIcon } from '@heroicons/react/24/outline';
import { cn } from '../../lib/utils';
import type { TableProps, TableColumn } from '../../types';

export function Table<T = any>({
  data,
  columns,
  loading = false,
  pagination,
  sortable = true,
  className,
  ...props
}: TableProps<T>) {
  const [sortConfig, setSortConfig] = React.useState<{
    key: string;
    direction: 'asc' | 'desc';
  } | null>(null);

  const handleSort = (column: TableColumn<T>) => {
    if (!column.sortable && !sortable) return;

    let direction: 'asc' | 'desc' = 'asc';
    if (sortConfig && sortConfig.key === column.key && sortConfig.direction === 'asc') {
      direction = 'desc';
    }

    setSortConfig({ key: column.key, direction });
  };

  const sortedData = React.useMemo(() => {
    if (!sortConfig) return data;

    return [...data].sort((a, b) => {
      const column = columns.find(col => col.key === sortConfig.key);
      if (!column) return 0;

      const aValue = column.dataIndex ? (a as any)[column.dataIndex] : (a as any)[sortConfig.key];
      const bValue = column.dataIndex ? (b as any)[column.dataIndex] : (b as any)[sortConfig.key];

      if (aValue < bValue) {
        return sortConfig.direction === 'asc' ? -1 : 1;
      }
      if (aValue > bValue) {
        return sortConfig.direction === 'asc' ? 1 : -1;
      }
      return 0;
    });
  }, [data, sortConfig, columns]);

  const getSortIcon = (column: TableColumn<T>) => {
    if (!sortConfig || sortConfig.key !== column.key) {
      return <ChevronUpIcon className="w-4 h-4 opacity-30" />;
    }
    return sortConfig.direction === 'asc' ? (
      <ChevronUpIcon className="w-4 h-4 text-brand-500" />
    ) : (
      <ChevronDownIcon className="w-4 h-4 text-brand-500" />
    );
  };

  return (
    <div className={cn('w-full', className)} {...props}>
      <div className="overflow-hidden rounded-lg border border-border-secondary bg-surface">
        <div className="overflow-x-auto">
          <table className="w-full">
            {/* Header */}
            <thead className="bg-surface-secondary">
              <tr>
                {columns.map((column) => (
                  <th
                    key={column.key}
                    className={cn(
                      'px-6 py-4 text-left text-sm font-semibold text-text-primary',
                      (column.sortable !== false && sortable) && 'cursor-pointer hover:bg-surface-tertiary',
                      column.align === 'center' && 'text-center',
                      column.align === 'right' && 'text-right'
                    )}
                    style={{ width: column.width }}
                    onClick={() => handleSort(column)}
                  >
                    <div className="flex items-center gap-2">
                      {typeof column.title === 'string' ? <span>{column.title}</span> : column.title}
                      {(column.sortable !== false && sortable) && getSortIcon(column)}
                    </div>
                  </th>
                ))}
              </tr>
            </thead>

            {/* Body */}
            <tbody className="divide-y divide-border-secondary">
              {loading ? (
                <tr>
                  <td colSpan={columns.length} className="px-6 py-12 text-center">
                    <div className="flex flex-col items-center gap-3">
                      <motion.div
                        className="w-8 h-8 border-2 border-brand-200 border-t-brand-500 rounded-full"
                        animate={{ rotate: 360 }}
                        transition={{ duration: 1, repeat: Infinity, ease: "linear" }}
                      />
                      <span className="text-text-secondary">Loading...</span>
                    </div>
                  </td>
                </tr>
              ) : sortedData.length === 0 ? (
                <tr>
                  <td colSpan={columns.length} className="px-6 py-12 text-center text-text-secondary">
                    No data available
                  </td>
                </tr>
              ) : (
                sortedData.map((row, index) => (
                  <motion.tr
                    key={index}
                    className="hover:bg-surface-secondary transition-colors"
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: index * 0.02 }}
                  >
                    {columns.map((column) => {
                      const value = column.dataIndex ? (row as any)[column.dataIndex] : (row as any)[column.key];
                      const cellContent = column.render 
                        ? column.render(value, row, index)
                        : value;

                      return (
                        <td
                          key={column.key}
                          className={cn(
                            'px-6 py-4 text-sm text-text-primary',
                            column.align === 'center' && 'text-center',
                            column.align === 'right' && 'text-right'
                          )}
                        >
                          {cellContent}
                        </td>
                      );
                    })}
                  </motion.tr>
                ))
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {pagination && (
          <div className="flex items-center justify-between px-6 py-4 border-t border-border-secondary bg-surface-secondary">
            <div className="flex items-center gap-2 text-sm text-text-secondary">
              <span>
                Showing {((pagination.current - 1) * pagination.pageSize) + 1} to{' '}
                {Math.min(pagination.current * pagination.pageSize, pagination.total)} of{' '}
                {pagination.total} entries
              </span>
            </div>
            <div className="flex items-center gap-2">
              <motion.button
                className="px-3 py-2 text-sm border border-border-secondary rounded-lg hover:bg-surface transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={pagination.current === 1}
                onClick={() => pagination.onChange(pagination.current - 1, pagination.pageSize)}
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
              >
                Previous
              </motion.button>
              <span className="px-3 py-2 text-sm text-text-primary">
                Page {pagination.current} of {Math.ceil(pagination.total / pagination.pageSize)}
              </span>
              <motion.button
                className="px-3 py-2 text-sm border border-border-secondary rounded-lg hover:bg-surface transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                disabled={pagination.current >= Math.ceil(pagination.total / pagination.pageSize)}
                onClick={() => pagination.onChange(pagination.current + 1, pagination.pageSize)}
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
              >
                Next
              </motion.button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}