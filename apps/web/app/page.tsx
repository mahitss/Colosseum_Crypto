export default function HomePage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-slate-50 px-6 py-16">
      <section className="w-full max-w-3xl rounded-2xl border border-slate-200 bg-white p-8 shadow-sm">
        <p className="mb-4 text-sm font-medium uppercase tracking-[0.2em] text-slate-500">
          Prophet
        </p>
        <h1 className="text-4xl font-bold tracking-tight text-slate-900">
          Prediction intelligence infrastructure
        </h1>
        <p className="mt-6 max-w-2xl text-lg text-slate-600">
          Building the foundational monorepo for a market intelligence platform connecting
          prediction data, signal generation, and AI-driven analysis.
        </p>
      </section>
    </main>
  );
}
