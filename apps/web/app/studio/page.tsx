'use client';

// Market Studio — the nine-step creation wizard.
//
// The rules this screen must never break:
//
//  1. The AI proposes; the human disposes. Nothing is created without an
//     explicit user action, and the resolution source is never decided for
//     the user.
//  2. The Create action is not rendered until deterministic validation passes
//     and the user has reviewed the content.
//  3. Warnings are always shown, and must be acknowledged before continuing.
//  4. This server never signs. Signing happens in the user's wallet.
//  5. A Solana signature is not a created market. Only Panta registration
//     makes a market exist, and the status text shown is the server's.

import { useCallback, useEffect, useMemo, useState } from 'react';

import {
  broadcastMarket,
  buildMarket,
  getMarketCreationAttempt,
  interpretMarket,
  MarketStudioError,
  quoteMarket,
  registerMarket,
  validateMarketDraft,
} from '@/lib/api-client';
import type {
  CreateBuild,
  CreateQuote,
  CreationAttempt,
  MarketDraft,
  ValidationReport,
} from '@/lib/market-studio-types';
import { PANTA_CATEGORIES } from '@/lib/market-studio-types';
import {
  canRetryBroadcast,
  canRetryRegistration,
  canShowCreateAction,
  changedMaterialFields,
  statusMessage,
  STEPS,
  STEP_INDEX,
  type StepId,
} from '@/lib/market-studio';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { useWallet } from '@/components/wallet-provider';
import { WizardStepper } from '@/components/market-studio/wizard-stepper';
import { QuoteSummary, ValidationPanel } from '@/components/market-studio/validation-panel';

const EMPTY_DRAFT: MarketDraft = {
  question: '',
  resolution_criteria: '',
  sources_of_truth: [],
  category: 'crypto',
  resolution_date: '',
  image_url: '',
  resolution_source_confirmed: false,
};

type ErrorState = { code: string; message: string; attempt?: CreationAttempt } | null;

export default function StudioPage() {
  const wallet = useWallet();

  const [step, setStep] = useState<StepId>('DESCRIBE');
  const [furthest, setFurthest] = useState<StepId>('DESCRIBE');
  const [unlockedThrough, setUnlockedThrough] = useState(0);

  const [prompt, setPrompt] = useState('');
  const [draft, setDraft] = useState<MarketDraft>(EMPTY_DRAFT);
  const [sourceInput, setSourceInput] = useState('');

  const [clarification, setClarification] = useState<string | null>(null);
  const [validation, setValidation] = useState<ValidationReport | null>(null);
  const [warningsAcknowledged, setWarningsAcknowledged] = useState(false);
  const [reviewed, setReviewed] = useState(false);

  const [quote, setQuote] = useState<CreateQuote | null>(null);
  const [build, setBuild] = useState<CreateBuild | null>(null);
  const [attempt, setAttempt] = useState<CreationAttempt | null>(null);
  // The exact draft the quote was taken against. Any material edit after this
  // point invalidates the quote and the built transaction.
  const [quotedDraft, setQuotedDraft] = useState<MarketDraft | null>(null);

  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ErrorState>(null);

  const goTo = useCallback(
    (next: StepId) => {
      const nextIndex = STEP_INDEX[next];
      if (nextIndex === undefined) return;
      setStep(next);
      if (nextIndex > STEP_INDEX[furthest]) {
        setFurthest(next);
        setUnlockedThrough((current) => Math.max(current, nextIndex));
      }
    },
    [furthest]
  );

  // Once a quote exists, any material edit invalidates it.
  const invalidatedFields = useMemo(
    () => (quotedDraft ? changedMaterialFields(quotedDraft, draft) : []),
    [quotedDraft, draft]
  );
  const quoteInvalidated = invalidatedFields.length > 0;

  const captureError = useCallback((err: unknown, fallbackCode: string) => {
    if (err instanceof MarketStudioError) {
      setError({ code: err.code, message: err.userMessage, attempt: err.attempt });
      if (err.attempt) setAttempt(err.attempt);
      return;
    }
    setError({ code: fallbackCode, message: 'Something went wrong. Try again.' });
  }, []);

  // ---- Step 1: describe -------------------------------------------------- //

  const handleInterpret = async () => {
    if (!prompt.trim()) return;
    setBusy(true);
    setError(null);
    setClarification(null);
    try {
      const result = await interpretMarket(prompt);
      if (result.needs_clarification) {
        // The question is too vague to become a market. Say so rather than
        // inventing the missing fields.
        setClarification(
          result.clarification ||
            'Your question needs more detail before a market can be created.'
        );
        goTo('CLARIFY');
        return;
      }
      if (result.draft) {
        setDraft({ ...EMPTY_DRAFT, ...result.draft });
        setValidation(result.validation ?? null);
        // A fresh interpretation has not been reviewed by a human yet.
        setReviewed(false);
        setWarningsAcknowledged(false);
        goTo('DRAFT');
      }
    } catch (err) {
      captureError(err, 'AI_INTERPRETATION_FAILED');
    } finally {
      setBusy(false);
    }
  };

  // ---- Step 5: validate -------------------------------------------------- //

  const handleValidate = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const report = await validateMarketDraft(draft);
      setValidation(report);
      setWarningsAcknowledged(false);
    } catch (err) {
      captureError(err, 'INVALID_DRAFT');
    } finally {
      setBusy(false);
    }
  }, [draft, captureError]);

  // Re-validate as the user edits so the Create action is never stale.
  // Debounced because this is a network round trip.
  useEffect(() => {
    if (STEP_INDEX[step] < STEP_INDEX.VALIDATE) return;
    if (!draft.question.trim()) return;
    const timer = setTimeout(handleValidate, 500);
    return () => clearTimeout(timer);
  }, [draft, step, handleValidate]);

  // ---- Step 6: quote ----------------------------------------------------- //

  const handleQuote = async () => {
    if (!wallet.publicKey) {
      setError({ code: 'WALLET_NOT_CONNECTED', message: 'Connect your wallet to continue.' });
      return;
    }
    if (!validation?.valid) {
      setError({ code: 'INVALID_DRAFT', message: 'Validation must pass before quoting.' });
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const result = await quoteMarket(draft, wallet.publicKey);
      setQuote(result.quote);
      setAttempt(result.attempt);
      setQuotedDraft(draft);
      // The user has now seen and accepted the exact content being created.
      setReviewed(true);
      goTo('QUOTE');
    } catch (err) {
      captureError(err, 'QUOTE_FAILED');
    } finally {
      setBusy(false);
    }
  };

  // ---- Steps 7, 8 and 9: build, sign, broadcast, register ----------------- //

  const handleBuildAndSign = async () => {
    if (!wallet.publicKey || !wallet.signTransaction) {
      setError({ code: 'WALLET_NOT_CONNECTED', message: 'Connect your wallet to continue.' });
      return;
    }
    if (quoteInvalidated) {
      setError({
        code: 'DRAFT_CHANGED_AFTER_QUOTE',
        message: `The market changed after it was quoted (${invalidatedFields.join(', ')}). Re-quote before continuing.`,
      });
      return;
    }
    if (!attempt?.id) {
      setError({ code: 'INVALID_DRAFT', message: 'Quote the market first.' });
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const result = await buildMarket(attempt.id, wallet.publicKey);
      setBuild(result.build);
      setAttempt(result.attempt);
      goTo('BUILD');

      // The wallet asks the user to sign. The server never holds a key.
      let signed: { serialize(options?: unknown): Uint8Array };
      try {
        signed = (await wallet.signTransaction(
          Buffer.from(result.build.transaction, 'base64')
        )) as { serialize(options?: unknown): Uint8Array };
      } catch (err) {
        const code = (err as { code?: number })?.code;
        const message = String((err as Error)?.message ?? '').toLowerCase();
        const rejected = code === 4001 || message.includes('reject');
        setError({
          code: rejected ? 'USER_REJECTED' : 'INVALID_TRANSACTION',
          message: rejected
            ? 'You rejected the signature request. Nothing was submitted.'
            : 'The transaction could not be signed. Nothing was submitted.',
          attempt: result.attempt,
        });
        return;
      }

      const broadcast = await broadcastMarket(
        result.attempt.id,
        Buffer.from(signed.serialize({ requireAllSignatures: false })).toString('base64'),
        wallet.publicKey,
        result.attempt.draft_hash
      );
      setAttempt(broadcast.attempt);
      goTo('BROADCAST');

      try {
        const registered = await registerMarket(broadcast.attempt.id);
        setAttempt(registered.attempt);
        goTo('REGISTER');
      } catch (err) {
        // Registration failing does not mean creation failed. The attempt is
        // preserved so the panel can report what actually happened.
        captureError(err, 'PANTA_REGISTRATION_FAILED');
        if (err instanceof MarketStudioError && err.attempt) goTo('REGISTER');
      }
    } catch (err) {
      captureError(err, 'BROADCAST_FAILED');
    } finally {
      setBusy(false);
    }
  };

  const handleRetryRegistration = async () => {
    if (!attempt) return;
    setBusy(true);
    setError(null);
    try {
      const registered = await registerMarket(attempt.id);
      setAttempt(registered.attempt);
    } catch (err) {
      captureError(err, 'PANTA_REGISTRATION_FAILED');
    } finally {
      setBusy(false);
    }
  };

  // Poll the authoritative state while an attempt is in flight. A failed poll is
  // not a failure: the last known state is kept.
  useEffect(() => {
    if (!attempt?.id) return;
    if (['REGISTERED', 'INDEXED', 'FAILED'].includes(attempt.status)) return;
    const timer = setInterval(async () => {
      try {
        setAttempt(await getMarketCreationAttempt(attempt.id));
      } catch {
        // Keep the last known state.
      }
    }, 4000);
    return () => clearInterval(timer);
  }, [attempt?.id, attempt?.status]);

  const stepIndex = STEP_INDEX[step];
  const showCreate = canShowCreateAction({
    step,
    validation,
    reviewed,
    warningsAcknowledged,
  });

  const updateDraft = (patch: Partial<MarketDraft>) => {
    setDraft((current) => ({ ...current, ...patch }));
    // Any edit invalidates a prior warning acknowledgement.
    setWarningsAcknowledged(false);
  };

  const addSource = () => {
    const value = sourceInput.trim();
    if (!value) return;
    if (!draft.sources_of_truth.includes(value)) {
      updateDraft({ sources_of_truth: [...draft.sources_of_truth, value] });
    }
    setSourceInput('');
  };

  const removeSource = (value: string) => {
    updateDraft({ sources_of_truth: draft.sources_of_truth.filter((s) => s !== value) });
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Market Studio</h1>
        <p className="mt-2 text-muted-foreground">
          Describe a market. The assistant drafts it, you review every detail, and
          only you decide whether it gets created.
        </p>
      </div>

      <WizardStepper
        current={step}
        furthest={furthest}
        onSelect={goTo}
        unlockedThrough={unlockedThrough}
      />

      {error && (
        <Card className="border-danger/40" data-testid="studio-error">
          <CardContent className="pt-6">
            <p className="font-medium text-danger">{error.message}</p>
            <p className="mt-1 text-xs text-muted-foreground">Code: {error.code}</p>
            {error.attempt && (
              <p className="mt-2 text-sm" data-testid="studio-error-attempt">
                {statusMessage(error.attempt)}
              </p>
            )}
          </CardContent>
        </Card>
      )}

      {/* ---- Step 1: describe -------------------------------------------- */}
      {step === 'DESCRIBE' && (
        <Card>
          <CardHeader>
            <CardTitle>Describe your market</CardTitle>
            <CardDescription>
              Write it in your own words. Be specific about what must be true, and when.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <Label htmlFor="prompt">Description</Label>
              <textarea
                id="prompt"
                value={prompt}
                onChange={(event) => setPrompt(event.target.value)}
                data-testid="studio-prompt"
                placeholder="Will Bitcoin's daily closing price be above 100,000 USD on 31 December 2026, according to CoinGecko?"
                className="mt-1 min-h-[120px] w-full rounded-md border border-input bg-background p-3 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              />
            </div>
            <Button
              onClick={handleInterpret}
              disabled={busy || !prompt.trim()}
              data-testid="studio-interpret"
            >
              {busy ? 'Thinking…' : 'Draft my market'}
            </Button>
            <p className="text-xs text-muted-foreground">
              This produces a draft for you to review. It does not create anything.
            </p>
          </CardContent>
        </Card>
      )}

      {/* ---- Step 2: clarification ---------------------------------------- */}
      {step === 'CLARIFY' && (
        <Card data-testid="studio-clarification">
          <CardHeader>
            <CardTitle>Your question needs more detail</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-sm">{clarification}</p>
            <p className="text-sm text-muted-foreground">
              A market needs a precise, checkable question, a clear resolution rule,
              and a named source. Add those details and try again.
            </p>
            <Button variant="outline" onClick={() => goTo('DESCRIBE')}>
              Back to description
            </Button>
          </CardContent>
        </Card>
      )}

      {/* ---- Steps 3 and 4: draft and resolution -------------------------- */}
      {(step === 'DRAFT' || step === 'RESOLUTION') && (
        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Review the draft</CardTitle>
              <CardDescription>
                Every field here came from the assistant. Correct anything that is
                wrong before continuing.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <Label htmlFor="question">Question</Label>
                <Input
                  id="question"
                  value={draft.question}
                  onChange={(event) => updateDraft({ question: event.target.value })}
                  data-testid="draft-question"
                  className="mt-1"
                />
              </div>
              <div>
                <Label htmlFor="title">Title (optional)</Label>
                <Input
                  id="title"
                  value={draft.title ?? ''}
                  onChange={(event) => updateDraft({ title: event.target.value })}
                  className="mt-1"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label htmlFor="category">Category</Label>
                  <select
                    id="category"
                    value={draft.category}
                    onChange={(event) => updateDraft({ category: event.target.value })}
                    data-testid="draft-category"
                    className="mt-1 h-10 w-full rounded-md border border-input bg-background px-3 text-sm"
                  >
                    {PANTA_CATEGORIES.map((category) => (
                      <option key={category} value={category}>
                        {category}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <Label htmlFor="resolution_date">Resolution date</Label>
                  <Input
                    id="resolution_date"
                    type="date"
                    value={draft.resolution_date}
                    onChange={(event) => updateDraft({ resolution_date: event.target.value })}
                    data-testid="draft-resolution-date"
                    className="mt-1"
                  />
                </div>
              </div>
              <div>
                <Label htmlFor="image_url">Image URL (required by Panta)</Label>
                <Input
                  id="image_url"
                  value={draft.image_url}
                  onChange={(event) => updateDraft({ image_url: event.target.value })}
                  data-testid="draft-image-url"
                  placeholder="https://example.com/market.png"
                  className="mt-1"
                />
              </div>
            </CardContent>
          </Card>

          {step === 'RESOLUTION' && (
            <Card>
              <CardHeader>
                <CardTitle>Resolution rules</CardTitle>
                <CardDescription>
                  These decide how the market settles. The assistant never sets them
                  without you.
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div>
                  <Label htmlFor="resolution_criteria">Resolution criteria</Label>
                  <textarea
                    id="resolution_criteria"
                    value={draft.resolution_criteria}
                    onChange={(event) => updateDraft({ resolution_criteria: event.target.value })}
                    data-testid="draft-resolution-criteria"
                    className="mt-1 min-h-[100px] w-full rounded-md border border-input bg-background p-3 text-sm"
                  />
                </div>

                <div>
                  <Label htmlFor="sources">Sources of truth</Label>
                  <div className="mt-1 flex gap-2">
                    <Input
                      id="sources"
                      value={sourceInput}
                      onChange={(event) => setSourceInput(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter') {
                          event.preventDefault();
                          addSource();
                        }
                      }}
                      data-testid="draft-source-input"
                      placeholder="https://www.coingecko.com/"
                    />
                    <Button type="button" variant="outline" onClick={addSource}>
                      Add
                    </Button>
                  </div>
                  {draft.sources_of_truth.length > 0 && (
                    <ul className="mt-2 space-y-1" data-testid="draft-sources">
                      {draft.sources_of_truth.map((source) => (
                        <li
                          key={source}
                          className="flex items-center justify-between rounded-md border border-border px-2 py-1 text-sm"
                        >
                          <span className="truncate">{source}</span>
                          <button
                            type="button"
                            onClick={() => removeSource(source)}
                            className="ml-2 text-xs text-muted-foreground hover:text-danger"
                          >
                            remove
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>

                <label className="flex items-start gap-2 rounded-md border border-border p-3 text-sm">
                  <input
                    type="checkbox"
                    checked={draft.resolution_source_confirmed}
                    onChange={(event) =>
                      updateDraft({ resolution_source_confirmed: event.target.checked })
                    }
                    data-testid="draft-confirm-source"
                    className="mt-0.5"
                  />
                  <span>
                    I confirm this resolution source is authoritative and I am
                    responsible for it.
                  </span>
                </label>
              </CardContent>
            </Card>
          )}

          <div className="flex gap-2">
            {step === 'DRAFT' && (
              <Button onClick={() => goTo('RESOLUTION')} data-testid="studio-to-resolution">
                Continue to resolution rules
              </Button>
            )}
            {step === 'RESOLUTION' && (
              <Button
                onClick={() => {
                  setReviewed(true);
                  goTo('VALIDATE');
                }}
                disabled={!draft.resolution_source_confirmed}
                data-testid="studio-to-validate"
              >
                I have reviewed this market
              </Button>
            )}
          </div>
        </div>
      )}

      {/* ---- Step 5: validate --------------------------------------------- */}
      {step === 'VALIDATE' && (
        <div className="space-y-4">
          <ValidationPanel
            report={validation}
            acknowledged={warningsAcknowledged}
            onAcknowledge={setWarningsAcknowledged}
          >
            {quoteInvalidated && (
              <p
                className="rounded-md border border-warning/40 bg-warning/5 p-2 text-sm"
                data-testid="quote-invalidated"
              >
                You changed {invalidatedFields.join(', ')} after the quote was taken.
                The previous quote and transaction no longer apply — re-quote to
                continue.
              </p>
            )}
          </ValidationPanel>

          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" onClick={handleValidate} disabled={busy}>
              Re-run validation
            </Button>

            {/* The Create action is not rendered at all until every gate passes.
                This is the single most important rule in this screen. */}
            {showCreate ? (
              <Button
                onClick={handleQuote}
                disabled={busy || !wallet.connected}
                data-testid="studio-create-market"
              >
                Create market
              </Button>
            ) : (
              <p className="text-sm text-muted-foreground" data-testid="create-blocked">
                The Create market action unlocks once validation passes and the market
                has been reviewed.
              </p>
            )}

            {!wallet.connected && (
              <Button variant="outline" onClick={() => wallet.connect()} data-testid="studio-connect">
                Connect wallet
              </Button>
            )}
          </div>
        </div>
      )}

      {/* ---- Step 6: quote ------------------------------------------------- */}
      {step === 'QUOTE' && (
        <Card>
          <CardHeader>
            <CardTitle>Fee quote</CardTitle>
            <CardDescription>
              Quoted by Panta. Nothing has been charged and no market exists yet.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            {quote && <QuoteSummary quote={quote} />}
            <div className="flex gap-2">
              <Button
                onClick={handleBuildAndSign}
                disabled={busy || quoteInvalidated || !quote}
                data-testid="studio-build-sign"
              >
                {busy ? 'Waiting for wallet…' : 'Build & sign in my wallet'}
              </Button>
              <Button variant="outline" onClick={() => goTo('VALIDATE')}>
                Back
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              You will be asked to approve the transaction in your wallet. Prophet
              never holds your keys.
            </p>
          </CardContent>
        </Card>
      )}

      {/* ---- Steps 7 and 8: build and broadcast ---------------------------- */}
      {(step === 'BUILD' || step === 'BROADCAST') && (
        <Card>
          <CardHeader>
            <CardTitle>
              {step === 'BUILD' ? 'Awaiting your signature' : 'Transaction submitted'}
            </CardTitle>
            <CardDescription>Waiting for Solana to confirm the transaction.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3">
            {attempt && (
              <div className="flex flex-wrap items-center gap-2" data-testid="studio-attempt">
                <Badge variant="outline">{attempt.status}</Badge>
                {attempt.solana_signature && (
                  <span className="truncate font-mono text-xs text-muted-foreground">
                    {attempt.solana_signature}
                  </span>
                )}
              </div>
            )}
            {build && (
              <p className="text-xs text-muted-foreground">
                Build fingerprint: <span className="font-mono">{build.buildFingerprint}</span>
              </p>
            )}
            <p className="text-sm">{attempt ? statusMessage(attempt) : ''}</p>
            <p className="text-xs text-muted-foreground">
              A confirmed transaction is not yet a market. It becomes one only when
              Panta registration succeeds.
            </p>
          </CardContent>
        </Card>
      )}

      {/* ---- Step 9: register and final state ------------------------------ */}
      {step === 'REGISTER' && attempt && (
        <Card data-testid="studio-final">
          <CardHeader>
            <CardTitle>Final state</CardTitle>
            <CardDescription>This is the server&apos;s own report, not an estimate.</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant={attempt.market_exists ? 'success' : 'warning'}>
                {attempt.status}
              </Badge>
              {attempt.market_id && <Badge variant="outline">{attempt.market_id}</Badge>}
            </div>

            <p className="text-sm" data-testid="studio-final-message">
              {statusMessage(attempt)}
            </p>

            {attempt.market_exists ? (
              <p className="text-sm text-muted-foreground">
                The market now exists on Panta. It will appear in Prophet once indexing
                catches up.
              </p>
            ) : (
              canRetryRegistration(attempt) && (
                <Button
                  onClick={handleRetryRegistration}
                  disabled={busy}
                  data-testid="studio-retry-register"
                >
                  Retry Panta registration
                </Button>
              )
            )}

            {canRetryBroadcast(attempt) && (
              <Button variant="outline" onClick={handleBuildAndSign} disabled={busy}>
                Retry transaction
              </Button>
            )}
          </CardContent>
        </Card>
      )}

      <p className="text-xs text-muted-foreground">
        Step {stepIndex + 1} of {STEPS.length}. Prophet does not create markets
        automatically and never signs on your behalf.
      </p>
    </div>
  );
}
