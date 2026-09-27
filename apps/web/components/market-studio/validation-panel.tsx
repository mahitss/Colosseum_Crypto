'use client';

import { AlertTriangle, CheckCircle2, Info } from 'lucide-react';

import { ValidationReport } from '@/lib/market-studio-types';
import { formatBaseUnits, isValidBaseUnits } from '@/lib/market-studio';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

interface ValidationPanelProps {
  report: ValidationReport | null;
  /** True once the user has ticked the box acknowledging the warnings. */
  acknowledged: boolean;
  onAcknowledge: (value: boolean) => void;
  children?: React.ReactNode;
}

/**
 * Shows every validation issue, warnings included.
 *
 * Warnings are never hidden and never silently cleared: the panel always lists
 * them, and the action below stays disabled until the user acknowledges them.
 * Suppressing a warning would let an unreviewed risk through.
 */
export function ValidationPanel({
  report,
  acknowledged,
  onAcknowledge,
  children,
}: ValidationPanelProps) {
  const issues = report?.issues ?? [];
  const errors = issues.filter((i) => i.severity === 'error');
  const warnings = issues.filter((i) => i.severity === 'warning');
  const missing = report?.missing_fields ?? [];

  if (!report) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Validation</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            Validation has not run yet. Nothing can be created until it passes.
          </p>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card data-testid="validation-panel">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          {report.valid ? (
            <>
              <CheckCircle2 className="h-4 w-4 text-success" aria-hidden="true" />
              Validation passed
            </>
          ) : (
            <>
              <AlertTriangle className="h-4 w-4 text-danger" aria-hidden="true" />
              Validation failed
            </>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {missing.length > 0 && (
          <div data-testid="missing-fields">
            <p className="text-sm font-medium">Still needed</p>
            <ul className="mt-1 list-inside list-disc text-sm text-muted-foreground">
              {missing.map((field) => (
                <li key={field}>{field}</li>
              ))}
            </ul>
          </div>
        )}

        {errors.length > 0 && (
          <ul className="space-y-2" data-testid="validation-errors">
            {errors.map((issue, index) => (
              <li
                key={`${issue.code}-${issue.field}-${index}`}
                className="flex items-start gap-2 rounded-md border border-danger/30 bg-danger/5 p-2 text-sm"
              >
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-danger" aria-hidden="true" />
                <span>
                  <span className="font-medium">{issue.message}</span>
                  <span className="ml-2 text-xs text-muted-foreground">{issue.field}</span>
                </span>
              </li>
            ))}
          </ul>
        )}

        {warnings.length > 0 && (
          <div data-testid="validation-warnings">
            <p className="flex items-center gap-2 text-sm font-medium">
              <Info className="h-4 w-4 text-warning" aria-hidden="true" />
              Warnings ({warnings.length})
            </p>
            <ul className="mt-2 space-y-2">
              {warnings.map((issue, index) => (
                <li
                  key={`${issue.code}-${issue.field}-${index}`}
                  className="flex items-start gap-2 rounded-md border border-warning/30 bg-warning/5 p-2 text-sm"
                >
                  <Info className="mt-0.5 h-4 w-4 shrink-0 text-warning" aria-hidden="true" />
                  <span>
                    <span className="font-medium">{issue.message}</span>
                    <span className="ml-2 text-xs text-muted-foreground">{issue.field}</span>
                  </span>
                </li>
              ))}
            </ul>
            <label className="mt-3 flex cursor-pointer items-start gap-2 text-sm">
              <input
                type="checkbox"
                checked={acknowledged}
                onChange={(event) => onAcknowledge(event.target.checked)}
                data-testid="acknowledge-warnings"
                className="mt-0.5"
              />
              <span>I have read these warnings and want to continue.</span>
            </label>
          </div>
        )}

        {children}
      </CardContent>
    </Card>
  );
}

interface QuoteSummaryProps {
  quote: {
    createId: string;
    expectedEventPda: string;
    paymentUsdc: string;
    liquidityInjectionUsdc: string;
    platformRevenueUsdc: string;
    marketType: string;
  };
}

/**
 * The fee breakdown exactly as Panta quoted it.
 *
 * Amounts are rendered by splitting the base-unit string, never by parsing it
 * into a number. The figure shown is therefore the figure that will be
 * charged — no rounding, no floating point drift, no estimate.
 */
export function QuoteSummary({ quote }: QuoteSummaryProps) {
  const rows: Array<[string, string]> = [
    ['Market creation payment', quote.paymentUsdc],
    ['Liquidity injection', quote.liquidityInjectionUsdc],
    ['Platform revenue', quote.platformRevenueUsdc],
  ];

  return (
    <div data-testid="quote-summary" className="space-y-3">
      <p className="text-sm text-muted-foreground">
        These amounts are quoted by Panta. They are not estimates.
      </p>
      <table className="w-full text-sm">
        <tbody>
          {rows.map(([label, amount]) => (
            <tr key={label} className="border-b border-border/50 last:border-0">
              <td className="py-1.5 text-muted-foreground">{label}</td>
              <td className="py-1.5 text-right font-mono">
                {isValidBaseUnits(amount) ? (
                  <>
                    {formatBaseUnits(amount)}{' '}
                    <span className="text-xs text-muted-foreground">USDC</span>
                  </>
                ) : (
                  <span className="text-muted-foreground">not quoted</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="flex flex-wrap gap-2 pt-1">
        <Badge variant="outline">{quote.marketType}</Badge>
        {quote.createId && <Badge variant="outline">createId {quote.createId}</Badge>}
      </div>
    </div>
  );
}
