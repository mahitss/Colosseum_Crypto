'use client';

import { STEPS, STEP_INDEX, type StepId } from '@/lib/market-studio';
import { cn } from '@/lib/cn';

interface WizardStepperProps {
  current: StepId;
  furthest: StepId;
  onSelect: (stepId: StepId) => void;
  /** Steps at or beyond this index are not reachable yet. */
  unlockedThrough: number;
}

/**
 * The nine-step progress indicator.
 *
 * Steps are selectable only up to `unlockedThrough`, so the user can revisit
 * any step they have actually reached but cannot jump ahead into a step whose
 * prerequisites have not been met.
 */
export function WizardStepper({
  current,
  furthest,
  onSelect,
  unlockedThrough,
}: WizardStepperProps) {
  const currentIndex = STEP_INDEX[current];
  const furthestIndex = STEP_INDEX[furthest];

  return (
    <nav aria-label="Market creation steps" className="w-full">
      <ol className="flex flex-wrap items-center gap-1">
        {STEPS.map((step, index) => {
          const isCurrent = index === currentIndex;
          const isDone = index < currentIndex;
          const isReachable = index <= unlockedThrough;
          const state = isCurrent ? 'current' : isDone ? 'done' : 'upcoming';

          return (
            <li key={step.id} className="flex items-center">
              <button
                type="button"
                onClick={() => isReachable && onSelect(step.id)}
                disabled={!isReachable}
                aria-current={isCurrent ? 'step' : undefined}
                data-testid={`step-${step.id.toLowerCase()}`}
                data-state={state}
                className={cn(
                  'flex items-center gap-2 rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors',
                  isCurrent && 'bg-primary text-primary-foreground',
                  !isCurrent && isReachable && 'text-foreground hover:bg-surface',
                  !isReachable && 'cursor-not-allowed text-muted-foreground/50'
                )}
              >
                <span
                  className={cn(
                    'flex h-5 w-5 items-center justify-center rounded-full border text-[10px]',
                    isCurrent && 'border-primary-foreground/40',
                    isDone && 'border-success bg-success text-white',
                    !isCurrent && !isDone && 'border-border'
                  )}
                >
                  {index + 1}
                </span>
                {step.label}
              </button>
              {index < STEPS.length - 1 && (
                <span
                  aria-hidden="true"
                  className={cn('mx-0.5 h-px w-3', index < furthestIndex ? 'bg-success' : 'bg-border')}
                />
              )}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
