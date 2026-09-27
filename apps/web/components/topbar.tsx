'use client';

import * as React from 'react';
import { useQuery } from '@tanstack/react-query';
import { getHealth } from '@/lib/api-client';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Search, Wallet, RefreshCw, LogOut } from 'lucide-react';
import { useRouter } from 'next/navigation';
import { useWallet } from '@/components/wallet-provider';

export function TopBar() {
  const router = useRouter();
  const [searchQuery, setSearchQuery] = React.useState('');
  const { available, connected, publicKey, connect, disconnect, connecting } = useWallet();

  const { data: health, isLoading, refetch } = useQuery({
    queryKey: ['health'],
    queryFn: getHealth,
    refetchInterval: 30000, // Check every 30 seconds
  });

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      router.push(`/markets?search=${encodeURIComponent(searchQuery)}`);
    }
  };

  const walletAddress = publicKey ?? '';
  const shortAddress = walletAddress
    ? `${walletAddress.slice(0, 4)}...${walletAddress.slice(-4)}`
    : '';

  return (
    <header className="h-14 bg-slate-900 border-b border-slate-800 flex items-center justify-between px-4">
      {/* Search */}
      <form onSubmit={handleSearch} className="flex-1 max-w-md">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
          <Input
            type="search"
            placeholder="Search markets... (Press /)"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-10 bg-slate-800 border-slate-700 text-white placeholder:text-slate-500 w-full"
          />
        </div>
      </form>

      {/* Right side */}
      <div className="flex items-center gap-4">
        {/* System Status */}
        <div className="flex items-center gap-2">
          <button
            onClick={() => refetch()}
            className="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-white transition-colors"
            title="Refresh status"
          >
            <RefreshCw className={`w-4 h-4 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
          <Badge 
            variant={getStatusVariant(health?.status)}
            className="text-xs"
          >
            {isLoading ? 'Checking...' : health?.status || 'Unknown'}
          </Badge>
        </div>

        {/* Wallet */}
        {!available ? (
          <Badge variant="secondary" className="text-xs">Wallet unavailable</Badge>
        ) : connected && publicKey ? (
          <div className="flex items-center gap-2">
            <Badge variant="success" className="text-xs font-mono">
              {shortAddress}
            </Badge>
            <Button
              variant="outline"
              size="sm"
              onClick={() => disconnect()}
              className="gap-2 text-slate-300 hover:text-white"
            >
              <LogOut className="w-4 h-4" />
              Disconnect
            </Button>
          </div>
        ) : (
          <Button
            variant="default"
            size="sm"
            onClick={() => connect()}
            disabled={connecting}
            className="gap-2"
          >
            <Wallet className="w-4 h-4" />
            {connecting ? 'Connecting...' : 'Connect Wallet'}
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