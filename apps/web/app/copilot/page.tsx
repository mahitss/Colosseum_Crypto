'use client';

import { useState } from 'react';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Plus, X } from 'lucide-react';

export default function CopilotPage() {
  const [messages, setMessages] = useState<any[]>([]);
  const [input, setInput] = useState('');

  const send = () => {
    if (!input.trim()) return;
    setMessages([...messages, {role: 'user', content: input }]);
    setInput('');
  };

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle>AI Copilot</CardTitle>
          <CardDescription>Ask Prophet about prediction markets.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="h-[350px] flex flex-col gap-2 overflow-auto" style={{background:'#f9f9fa'}}>
            {messages.map((m,i)=>(
              <div key={i} className={m.role==='user'?'text-right':''}>
                <div className={m.role==='user'?'ml-auto inline-block rounded-md bg-primary p-2 text-primary-foreground':'inline-block rounded-md bg-muted-foreground p-2'}>{m.content}</div>
              </div>
            ))}
          </div>
          <div className="mt-4">
            <Input
              placeholder="Ask Prophet..."
              value={input}
              onChange={e=>setInput(e.target.value)}
              onKeyDown={e=>e.key==='Enter'?send():undefined}
              className="border"/>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
