'use client';

import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/cn';

const skeletonVariants = cva('animate-pulse rounded-md bg-muted');

interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'default' | 'text' | 'circle' | 'card';
}

const Skeleton = React.forwardRef<HTMLDivElement, SkeletonProps>(
  ({ className, variant = 'default', ...props }, ref) => {
    const variantClasses = {
      default: 'h-4 w-full',
      text: 'h-4 w-3/4',
      circle: 'h-10 w-10 rounded-full',
      card: 'h-48 w-full',
    };

    return (
      <div
        className={cn(skeletonVariants, variantClasses[variant], className)}
        ref={ref}
        {...props}
      />
    );
  }
);
Skeleton.displayName = 'Skeleton';

export { Skeleton };
