"use client";

import * as React from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { formatDistanceToNow } from "date-fns";
import { Plus, Trash2, ChevronRight, Clock, AlertTriangle } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent, CardFooter } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import type { WatchlistSummary } from "@/lib/api-types";

export interface WatchlistsContentProps {
  initialWatchlists: WatchlistSummary[];
}

export function WatchlistsContent({ initialWatchlists }: WatchlistsContentProps) {
  const queryClient = useQueryClient();
  const [showCreateForm, setShowCreateForm] = React.useState(false);
  const [createName, setCreateName] = React.useState("");
  const [createDescription, setCreateDescription] = React.useState("");

  const { data: watchlists = [], isLoading, error, refetch } = useQuery({
    queryKey: ["watchlists"],
    queryFn: async () => {
      const response = await fetch("/api/watchlists");
      if (!response.ok) throw new Error("Failed to fetch watchlists");
      return response.json() as Promise<WatchlistSummary[]>;
    },
    initialData: initialWatchlists,
    staleTime: 30_000,
  });

  const createMutation = useMutation({
    mutationFn: async (input: { name: string; description?: string | null }) => {
      const response = await fetch("/api/watchlists", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      });
      if (!response.ok) throw new Error("Failed to create watchlist");
      return response.json() as Promise<WatchlistSummary>;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["watchlists"] });
      setShowCreateForm(false);
      setCreateName("");
      setCreateDescription("");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: async (id: string) => {
      const response = await fetch(`/api/watchlists/${encodeURIComponent(id)}`, {
        method: "DELETE",
      });
      if (!response.ok) throw new Error("Failed to delete watchlist");
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["watchlists"] });
    },
  });

  const handleCreateSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!createName.trim()) return;
    createMutation.mutate({ name: createName.trim(), description: createDescription.trim() || null });
  };

  const handleDelete = (id: string) => {
    if (window.confirm("Are you sure you want to delete this watchlist?")) {
      deleteMutation.mutate(id);
    }
  };

  const getSeverityVariant = (severity: string | null): "info" | "warning" | "success" | "critical" | "secondary" => {
    if (!severity) return "secondary";
    switch (severity.toUpperCase()) {
      case "INFO":
        return "info";
      case "WATCH":
        return "warning";
      case "SIGNIFICANT":
        return "success";
      case "CRITICAL":
        return "critical";
      default:
        return "secondary";
    }
  };

  const formatTimestamp = (timestamp: string | null) => {
    if (!timestamp) return "Never";
    try {
      return formatDistanceToNow(new Date(timestamp), { addSuffix: true });
    } catch {
      return "Unknown";
    }
  };

  if (isLoading && watchlists.length === 0) {
    return (
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <Card key={i}>
            <CardContent className="p-6">
              <div className="space-y-3">
                <Skeleton className="h-6 w-3/4" />
                <Skeleton className="h-4 w-1/2" />
                <Skeleton className="h-4 w-1/3" />
              </div>
            </CardContent>
          </Card>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <ErrorState
        title="Unable to load watchlists"
        description="There was an error fetching your watchlists. Please try again."
        onRetry={() => refetch()}
      />
    );
  }

  return (
    <div className="space-y-6">
      {/* Create Watchlist Form */}
      {showCreateForm ? (
        <Card>
          <form onSubmit={handleCreateSubmit} className="p-6 space-y-4">
            <div className="flex items-center justify-between">
              <CardTitle className="text-lg">Create Watchlist</CardTitle>
              <Button type="button" variant="ghost" size="icon" onClick={() => setShowCreateForm(false)}>
                <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" /></svg>
              </Button>
            </div>
            <div className="space-y-3">
              <div>
                <label htmlFor="watchlist-name" className="block text-sm font-medium mb-1">Name</label>
                <Input
                  id="watchlist-name"
                  placeholder="Enter watchlist name..."
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                  required
                  autoFocus
                />
              </div>
              <div>
                <label htmlFor="watchlist-description" className="block text-sm font-medium mb-1">Description (optional)</label>
                <Input
                  id="watchlist-description"
                  placeholder="Enter description..."
                  value={createDescription}
                  onChange={(e) => setCreateDescription(e.target.value)}
                />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <Button type="button" variant="outline" onClick={() => setShowCreateForm(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={createMutation.isPending || !createName.trim()}>
                {createMutation.isPending ? "Creating..." : "Create Watchlist"}
              </Button>
            </div>
          </form>
        </Card>
      ) : (
        <Card>
          <CardContent className="p-6">
            <Button onClick={() => setShowCreateForm(true)} className="w-full sm:w-auto">
              <Plus className="h-4 w-4 mr-2" />
              Create Watchlist
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Watchlists Grid */}
      {watchlists.length === 0 && !showCreateForm ? (
        <Card>
          <CardContent className="py-12">
            <EmptyState
              title="No watchlists yet"
              description="Create your first watchlist to start tracking markets and monitoring signals."
              icon={<AlertTriangle className="h-12 w-12 text-muted-foreground/50" />}
            />
          </CardContent>
        </Card>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {watchlists.map((watchlist) => (
            <WatchlistCard
              key={watchlist.id}
              watchlist={watchlist}
              onDelete={handleDelete}
              getSeverityVariant={getSeverityVariant}
              formatTimestamp={formatTimestamp}
            />
          ))}
        </div>
      )}
    </div>
  );
}

interface WatchlistCardProps {
  watchlist: WatchlistSummary;
  onDelete: (id: string) => void;
  getSeverityVariant: (severity: string | null) => "info" | "warning" | "success" | "critical" | "secondary";
  formatTimestamp: (timestamp: string | null) => string;
}

function WatchlistCard({
  watchlist,
  onDelete,
  getSeverityVariant,
  formatTimestamp,
}: WatchlistCardProps) {
  const severityVariant = getSeverityVariant(watchlist.latest_signal_severity);

  return (
    <Link href={`/watchlists/${watchlist.id}`} className="block">
      <Card className="h-full hover:shadow-md transition-shadow cursor-pointer">
        <CardHeader className="pb-3">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0 flex-1">
              <CardTitle className="text-lg truncate">{watchlist.name}</CardTitle>
              {watchlist.description && (
                <p className="text-sm text-muted-foreground mt-1 line-clamp-2">{watchlist.description}</p>
              )}
            </div>
            <div className="flex items-center gap-1 flex-shrink-0">
              <Button
                variant="ghost"
                size="icon"
                className="text-muted-foreground hover:text-danger hover:bg-danger/10"
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  onDelete(watchlist.id);
                }}
              >
                <Trash2 className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent className="space-y-3 pb-2">
          <div className="flex items-center gap-3 text-sm">
            <div className="flex items-center gap-1.5 text-muted-foreground">
              <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" /></svg>
              <span className="font-medium">{watchlist.market_count}</span>
              <span className="text-muted-foreground">markets</span>
            </div>
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            {watchlist.latest_signal_severity ? (
              <Badge variant={severityVariant} className="gap-1">
                {watchlist.latest_signal_severity}
                {watchlist.latest_signal_at && (
                  <>
                    <Clock className="h-3 w-3" />
                    <span className="text-xs">{formatTimestamp(watchlist.latest_signal_at)}</span>
                  </>
                )}
              </Badge>
            ) : (
              <Badge variant="secondary" className="gap-1">
                <Clock className="h-3 w-3" />
                <span className="text-xs">No signals</span>
              </Badge>
            )}
            {watchlist.updated_at && (
              <Badge variant="outline" className="gap-1 text-xs">
                <Clock className="h-3 w-3" />
                <span>Updated {formatTimestamp(watchlist.updated_at)}</span>
              </Badge>
            )}
          </div>
        </CardContent>
        <CardFooter className="pt-0">
          <div className="flex items-center justify-between w-full">
            <span className="text-xs text-muted-foreground">View details</span>
            <ChevronRight className="h-4 w-4 text-muted-foreground" />
          </div>
        </CardFooter>
      </Card>
    </Link>
  );
}
