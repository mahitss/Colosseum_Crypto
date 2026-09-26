'use client';

import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/cn';

interface EmptyStateProps extends React.HTMLAttributes<HTMLDivElement> {
  title?: string;
  description?: string;
  icon?: React.ReactNode;
}

const EmptyState = React.forwardRef<HTMLDivElement, EmptyStateProps>(
  ({ className, title = 'No data available', description, icon, ...props }, ref) => {
    return (
      <div
        className={cn('flex flex-col items-center justify-center py-12 text-center', className)}
        ref={ref}
        {...props}
      >
        {icon && <div className="mb-4 text-muted-foreground">{icon}</div>}
        <h3 className="text-lg font-semibold text-foreground">{title}</h3>
        {description && <p className="mt-2 text-sm text-muted-foreground max-w-sm">{description}</p>}
      </div>
    );
  }
);
EmptyState.displayName = 'EmptyState';

export { EmptyState };
