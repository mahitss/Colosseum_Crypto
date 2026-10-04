'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/cn';
import {
  LayoutDashboard,
  TrendingUp,
  Signal,
  Bookmark,
  Wallet,
  Bot,
  PenTool,
  Settings,
  BellRing,
  Activity,
  Search,
  HelpCircle,
} from 'lucide-react';

const navItems = [
  { href: '/', label: 'Command Center', icon: LayoutDashboard },
  { href: '/markets', label: 'Markets', icon: TrendingUp },
  { href: '/signals', label: 'Signal Radar', icon: Signal },
  { href: '/watchlists', label: 'Watchlists', icon: Bookmark },
  { href: '/portfolio', label: 'Portfolio', icon: Wallet },
  { href: '/copilot', label: 'AI Copilot', icon: Bot },
  { href: '/studio', label: 'Market Studio', icon: PenTool },
  { href: '/settings/alerts', label: 'Alerts', icon: BellRing },
];

const bottomNavItems = [
  { href: '/settings', label: 'Settings', icon: Settings },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="w-64 bg-slate-900/80 backdrop-blur-sm border-r border-slate-800 flex flex-col h-screen sticky top-0 z-40">
      {/* Logo */}
      <div className="h-16 flex items-center px-6 border-b border-slate-800/60">
        <Link href="/" className="flex items-center gap-3">
          <div className="w-8 h-8 bg-emerald-500/20 rounded-lg flex items-center justify-center border border-emerald-500/30">
            <Activity className="w-5 h-5 text-emerald-400" />
          </div>
          <span className="text-lg font-bold text-white tracking-tight">QEVRYN</span>
        </Link>
      </div>

      {/* Main Navigation */}
      <nav className="flex-1 py-4 px-3 space-y-1 overflow-y-auto">
        {navItems.map((item) => {
          const isActive = pathname === item.href || 
            (item.href !== '/' && pathname.startsWith(item.href));
          
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-100',
                isActive
                  ? 'bg-emerald-500/10 text-white border border-emerald-500/30'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800/50'
              )}
            >
              <item.icon className={cn('w-5 h-5 shrink-0', isActive && 'text-emerald-400')} />
              {item.label}
            </Link>
          );
        })}
      </nav>

      {/* Bottom Navigation */}
      <div className="py-4 px-3 border-t border-slate-800/60 space-y-1">
        {bottomNavItems.map((item) => {
          const isActive = pathname === item.href;
          
          return (
            <Link
              key={item.href}
              href={item.href}
              className={cn(
                'flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-100',
                isActive
                  ? 'bg-slate-800/50 text-white'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800/50'
              )}
            >
              <item.icon className="w-5 h-5 shrink-0" />
              {item.label}
            </Link>
          );
        })}
        
        {/* System Status Link */}
        <Link
          href="/system-status"
          className="flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-all duration-100 text-slate-400 hover:text-white hover:bg-slate-800/50"
        >
          <HelpCircle className="w-5 h-5 shrink-0 text-slate-500" />
          <span>System Status</span>
        </Link>
      </div>
    </aside>
  );
}