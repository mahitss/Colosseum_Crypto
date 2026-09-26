'use client';

import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/cn';

const tableVariants = cva('w-full caption-bottom text-sm', {
  variants: {
    variant: {
      default: 'border-border',
      card: 'border-0',
    },
  },
  defaultVariants: {
    variant: 'default',
  },
});

const tableHeaderVariants = cva('h-12 px-4 text-left align-middle font-medium text-muted-foreground', {
  variants: {
    variant: {
      default: 'border-b border-border',
      card: 'border-0',
    },
  },
  defaultVariants: {
    variant: 'default',
  },
});

const tableBodyVariants = cva('[&_tr:last-child]:border-0');

const tableRowVariants = cva('transition-colors hover:bg-surface/50 data-[state=selected]:bg-surface');

const tableCellVariants = cva('p-4 align-middle [&:has([role=checkbox])]:align-middle [&:has([role=checkbox])]:translate-y-0.5', {
  variants: {
    variant: {
      default: 'border-b border-border',
      card: 'border-0',
    },
  },
  defaultVariants: {
    variant: 'default',
  },
});

export interface TableProps extends React.HTMLAttributes<HTMLTableElement>, VariantProps<typeof tableVariants> {}

const Table = React.forwardRef<HTMLTableElement, TableProps>(
  ({ className, variant, ...props }, ref) => {
    return (
      <div className="relative w-full overflow-auto">
        <table ref={ref} className={cn(tableVariants({ variant, className }))} {...props} />
      </div>
    );
  }
);
Table.displayName = 'Table';

const TableHeader = React.forwardRef<HTMLTableSectionElement, React.HTMLAttributes<HTMLTableSectionElement>>(
  ({ className, ...props }, ref) => {
    return <thead className={cn(tableHeaderVariants({ className }))} ref={ref} {...props} />;
  }
);
TableHeader.displayName = 'TableHeader';

const TableBody = React.forwardRef<HTMLTableSectionElement, React.HTMLAttributes<HTMLTableSectionElement>>(
  ({ className, ...props }, ref) => {
    return <tbody className={cn(tableBodyVariants({ className }))} ref={ref} {...props} />;
  }
);
TableBody.displayName = 'TableBody';

const TableRow = React.forwardRef<HTMLTableRowElement, React.HTMLAttributes<HTMLTableRowElement>>(
  ({ className, ...props }, ref) => {
    return (
      <tr className={cn(tableRowVariants({ className }))} ref={ref} {...props} />
    );
  }
);
TableRow.displayName = 'TableRow';

const TableCell = React.forwardRef<HTMLTableCellElement, React.HTMLAttributes<HTMLTableCellElement>>(
  ({ className, ...props }, ref) => {
    return (
      <td className={cn(tableCellVariants({ className }))} ref={ref} {...props} />
    );
  }
);
TableCell.displayName = 'TableCell';

const TableFooter = React.forwardRef<HTMLTableSectionElement, React.HTMLAttributes<HTMLTableSectionElement>>(
  ({ className, ...props }, ref) => {
    return <tfoot className={cn('border-t border-border font-medium [&>tr]:last:border-b-0', className)} ref={ref} {...props} />;
  }
);
TableFooter.displayName = 'TableFooter';

const TableHead = React.forwardRef<HTMLTableCellElement, React.HTMLAttributes<HTMLTableCellElement>>(
  ({ className, ...props }, ref) => {
    return <th className={cn('h-12 px-4 text-left align-middle font-medium text-muted-foreground [&:has([role=checkbox])]:pl-3', className)} ref={ref} {...props} />;
  }
);
TableHead.displayName = 'TableHead';

const TableCaption = React.forwardRef<HTMLTableCaptionElement, React.HTMLAttributes<HTMLTableCaptionElement>>(
  ({ className, ...props }, ref) => {
    return <caption className={cn('mt-4 text-sm text-muted-foreground', className)} ref={ref} {...props} />;
  }
);
TableCaption.displayName = 'TableCaption';

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
};
