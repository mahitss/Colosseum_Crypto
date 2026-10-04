'use client';

import * as React from 'react';
import { useQuery } from '@tanstack/react-query';
import { getHealth } from '@/lib/api-client';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Search, Wallet, RefreshCw, LogOut, Command } from 'lucide-react';
import { useRouter } from 'next/navigation';
import { useWallet } from '@/components/wallet-provider';
import { NotificationCenter } from '@/components/notification-center';
import { cn } from '@/lib/cn';

export function TopBar() {
  const router = useRouter();
  const [searchQuery, setSearchQuery] = React.useState('');
  const [showSearch, setShowSearch] = React.useState(false);
  const { available, connected, publicKey, connect, disconnect, connecting } = useWallet();

  const { data: health, isLoading, refetch } = useQuery({
    queryKey: ['health'],
    queryFn: getHealth,
    refetchInterval: 30000,
  });

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      router.push(`/markets?search=${encodeURIComponent(searchQuery)}`);
      setShowSearch(false);
    }
  };

  const walletAddress = publicKey ?? '';
  const shortAddress = walletAddress
    ? `${walletAddress.slice(0, 4)}...${walletAddress.slice(-4)}`
    : '';

  // Keyboard shortcut: Cmd/Ctrl + K for search
  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setShowSearch(true);
      }
      if (e.key === 'Escape') {
        setShowSearch(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  return (
    <header className="h-14 bg-slate-900/80 backdrop-blur-sm border-b border-slate-800/60 sticky top-0 z-30 flex items-center justify-between px-4">
      {/* Search */}
      <div className={cn('relative flex-1 max-w-md', showSearch && 'max-w-lg')}>
        <form onSubmit={handleSearch} className="relative w-full">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
          <Input
            type="search"
            placeholder={showSearch ? 'Search markets, signals, watchlists... (Press /)' : 'Search markets... (Press ⌘K)'}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onFocus={() => setShowSearch(true)}
            onBlur={(e) => {
              // Delay to allow click on results
              setTimeout(() => setShowSearch(false), 150);
            }}
            className="pl-10 bg-slate-800/50 border-slate-700/50 text-white placeholder:text-slate-500 w-full focus:border-emerald-500/50 focus:ring-1 focus:ring-emerald-500/20"
          />
          <div className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-500 text-[10px] font-mono">
            ⌘K
          </div>
          {showSearch && (
            <div className="absolute top-full left-0 right-0 mt-1 bg-slate-900 border border-slate-800 rounded-lg shadow-xl p-2 z-50">
              <p className="text-xs text-slate-500 px-2 py-1">Press ⌘K to search markets, signals, and watchlists</p>
            </div>
          )}
        </form>
      </div>

      {/* Right side */}
      <div className="flex items-center gap-3">
        {/* System Status */}
        <div className="flex items-center gap-2">
          <button
            onClick={() => refetch()}
            className="p-1.5 rounded hover:bg-slate-800/50 text-slate-400 hover:text-white transition-colors"
            title="Refresh status"
            aria-label="Refresh system status"
          >
            <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
          <Badge 
            variant={getStatusVariant(health?.status)}
            className="text-[11px] font-medium"
          >
            {isLoading ? 'Checking...' : health?.status || 'Unknown'}
          </Badge>
        </div>

        {/* Notifications */}
        <NotificationCenter />

        {/* Wallet */}
        {!available ? (
          <Badge variant="secondary" className="text-[11px]">Wallet unavailable</Badge>
        ) : connected && publicKey ? (
          <div className="flex items-center gap-2">
            <Badge variant="success" className="text-[11px] font-mono">
              {shortAddress}
            </Badge>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => disconnect()}
              className="gap-1.5 text-slate-300 hover:text-white"
            >
              <LogOut className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Disconnect</span>
            </Button>
          </div>
        ) : (
          <Button
            variant="default"
            size="sm"
            onClick={() => connect()}
            disabled={connecting}
            className="gap-1.5"
          >
            <Wallet className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">{connecting ? 'Connecting...' : 'Connect Wallet'}</span>
          </Button>
        )}
      </div>
    </header>
  );
}

function getStatusVariant(status?: string): 'success' | 'warning' | 'destructive' | 'secondary' {
  switch (status) {
    case 'operational':
      return 'success';
    case 'degraded':
      return 'warning';
    case 'unknown':
      return 'secondary';
    default:
      return 'destructive';
  }
}

// Debounce hook for search
function useDebouncedValue<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = React.useState(value);

  React.useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return debounced;
}