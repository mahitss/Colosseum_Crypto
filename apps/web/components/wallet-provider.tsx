'use client';

// Solana wallet provider with graceful degradation.
//
// When the @solana/* adapter packages are installed, the real
// ConnectionProvider / WalletProvider / WalletModalProvider tree is mounted and
// the app exposes a live `useWallet()` hook. When the packages cannot be
// imported (e.g. interrupted install), the app builds and renders an honest
// "wallet unavailable" state — never a fake connected wallet.

import {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  ReactNode,
} from 'react';

export interface WalletAPI {
  available: boolean;
  loading: boolean;
  connected: boolean;
  connecting: boolean;
  publicKey: string | null;
  signTransaction: ((tx: unknown) => Promise<{ serialize(opt?: unknown): Uint8Array }>) | null;
  connect: () => Promise<void>;
  disconnect: () => Promise<void>;
}

const UNAVAILABLE: WalletAPI = {
  available: false,
  loading: false,
  connected: false,
  connecting: false,
  publicKey: null,
  signTransaction: null,
  connect: async () => {},
  disconnect: async () => {},
};

const WalletContext = createContext<WalletAPI>(UNAVAILABLE);
export function useWallet(): WalletAPI {
  return useContext(WalletContext);
}

// Lazy-loaded adapter modules (only present when packages are installed).
interface AdapterModules {
  ConnectionProvider: (props: { endpoint: string; children: ReactNode }) => ReactNode;
  WalletProvider: (props: { wallets: unknown[]; children: ReactNode; autoConnect?: boolean }) => ReactNode;
  WalletModalProvider: (props: { children: ReactNode }) => ReactNode;
  useWallet: () => {
    connected: boolean;
    connecting: boolean;
    publicKey: { toBase58(): string } | null;
    signTransaction?: (tx: unknown) => Promise<{ serialize(opt?: unknown): Uint8Array }>;
    connect: () => Promise<void>;
    disconnect: () => Promise<void>;
  };
  wallets: unknown[];
}

const WalletBridge = ({ inner, adapters }: { inner: ReactNode; adapters: AdapterModules }) => {
  const w = adapters.useWallet();
  const value: WalletAPI = {
    available: true,
    loading: false,
    connected: w.connected,
    connecting: w.connecting,
    publicKey: w.publicKey ? w.publicKey.toBase58() : null,
    signTransaction: (w.signTransaction as WalletAPI['signTransaction']) || null,
    connect: useCallback(() => w.connect(), [w]),
    disconnect: useCallback(() => w.disconnect(), [w]),
  };
  return <WalletContext.Provider value={value}>{inner}</WalletContext.Provider>;
};

export function WalletProviderConfig({ children }: { children: ReactNode }) {
  const [adapters, setAdapters] = useState<AdapterModules | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [react, reactUI, wallets] = await Promise.all([
          import('@solana/wallet-adapter-react'),
          import('@solana/wallet-adapter-react-ui'),
          import('@solana/wallet-adapter-wallets'),
        ]);
        if (cancelled) return;
        setAdapters({
          ConnectionProvider: react.ConnectionProvider,
          WalletProvider: react.WalletProvider,
          WalletModalProvider: reactUI.WalletModalProvider,
          useWallet: react.useWallet,
          wallets: [
            new wallets.PhantomWalletAdapter(),
            new wallets.SolflareWalletAdapter(),
            new wallets.BackpackWalletAdapter(),
          ],
        });
      } catch {
        if (!cancelled) setAdapters(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);
  
  if (!adapters) {
    return (
      <WalletContext.Provider value={{ ...UNAVAILABLE, loading }}>
        {children}
      </WalletContext.Provider>
    );
  }

  const endpoint =
    process.env.NEXT_PUBLIC_SOLANA_RPC_URL ||
    (process.env.NEXT_PUBLIC_SOLANA_NETWORK === 'devnet'
      ? 'https://api.devnet.solana.com'
      : 'https://api.mainnet-beta.solana.com');

  return (
    <adapters.ConnectionProvider endpoint={endpoint}>
      <adapters.WalletProvider wallets={adapters.wallets} autoConnect={false}>
        <adapters.WalletModalProvider>
          <WalletBridge adapters={adapters} inner={children} />
        </adapters.WalletModalProvider>
      </adapters.WalletProvider>
    </adapters.ConnectionProvider>
  );
}