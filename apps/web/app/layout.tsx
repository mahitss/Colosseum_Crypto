import type { Metadata } from 'next';
import './globals.css';
import { Providers } from '@/components/providers';
import { Sidebar } from '@/components/sidebar';
import { TopBar } from '@/components/topbar';
import { WalletProviderConfig } from '@/components/wallet-provider';

export const metadata: Metadata = {
  title: 'QEVRYN — Enterprise Prediction Intelligence',
  description: 'See what the market thinks happens next. Enterprise prediction intelligence powered by Panta.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <body className="bg-slate-950 min-h-screen">
        <Providers>
          <WalletProviderConfig>
            <div className="flex h-screen overflow-hidden">
              <Sidebar />
              <div className="flex-1 flex flex-col overflow-hidden">
                <TopBar />
                <main className="flex-1 overflow-auto p-6 bg-slate-950">
                  {children}
                </main>
              </div>
            </div>
          </WalletProviderConfig>
        </Providers>
      </body>
    </html>
  );
}
