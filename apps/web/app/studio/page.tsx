'use client';

import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

export default function StudioPage() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Market Studio</h1>
        <p className="text-muted-foreground mt-2">Create and manage prediction markets.</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Create Market</CardTitle>
          <CardDescription>Submit a new prediction market to the Panta ecosystem.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <Label htmlFor="question">Question</Label>
            <Input id="question" placeholder="Will BTC reach $150K by end of 2026?" className="mt-1" />
          </div>
          <div>
            <Label htmlFor="description">Description</Label>
            <Input id="description" placeholder="Optional description..." className="mt-1" />
          </div>
          <div>
            <Label htmlFor="criteria">Resolution Criteria</Label>
            <Input id="criteria" placeholder="How will this market resolve?" className="mt-1" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <Label htmlFor="resolutionDate">Resolution Date</Label>
              <Input id="resolutionDate" type="date" className="mt-1" />
            </div>
            <div>
              <Label htmlFor="category">Category</Label>
              <Input id="category" placeholder="Crypto, Sports, etc." className="mt-1" />
            </div>
          </div>
          <Button className="w-full">Review Market</Button>
        </CardContent>
      </Card>

      <p className="text-xs text-muted-foreground">Market creation will be available in a future update.</p>
    </div>
  );
}