// Type shims for optional Solana packages.
//
// The @solana/* packages are declared in package.json. On systems where they
// are installed, the real modules provide these types. When they are not
// installed (e.g. interrupted install), these ambient declarations keep
// TypeScript compiling so the app can degrade gracefully at runtime.

declare module '@solana/web3.js' {
  export class Transaction {
    static from(data: Uint8Array | string): Transaction;
    serialize(opts?: { requireAllSignatures?: boolean }): Uint8Array;
    signature?: Uint8Array | null;
  }
  export class PublicKey {
    constructor(value: string | Uint8Array);
    toBase58(): string;
    equals(other: PublicKey): boolean;
  }
  export class Connection {
    constructor(endpoint: string, commitment?: unknown);
    sendRawTransaction(raw: Uint8Array): Promise<string>;
    getSignatureStatuses(sigs: string[]): Promise<unknown>;
    getLatestBlockhash(): Promise<unknown>;
  }
  export function clusterApiUrl(network: string): string;
}

declare module '@solana/wallet-adapter-base' {
  export class BaseWalletAdapter {}
}

interface SerializedTransaction {
  serialize(opt?: { requireAllSignatures?: boolean }): Uint8Array;
}

declare module '@solana/wallet-adapter-react' {
  export function useWallet(): {
    connected: boolean;
    connecting: boolean;
    publicKey: { toBase58(): string } | null;
    signTransaction?: (tx: unknown) => Promise<SerializedTransaction>;
    sendTransaction?: (tx: unknown, connection: unknown) => Promise<string>;
    connect: () => Promise<void>;
    disconnect: () => Promise<void>;
    wallets: unknown[];
  };
  export function ConnectionProvider(props: { endpoint: string; children: React.ReactNode }): React.ReactNode;
  export function WalletProvider(props: { wallets: unknown[]; children: React.ReactNode; autoConnect?: boolean }): React.ReactNode;
}

declare module '@solana/wallet-adapter-react-ui' {
  export function WalletModalProvider(props: { children: React.ReactNode }): React.ReactNode;
  export function WalletMultiButton(): React.ReactNode;
}

declare module '@solana/wallet-adapter-wallets' {
  export class PhantomWalletAdapter {
    constructor(opts?: unknown);
  }
  export class SolflareWalletAdapter {
    constructor(opts?: unknown);
  }
  export class BackpackWalletAdapter {
    constructor(opts?: unknown);
  }
}