'use client';

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { formatDistanceToNow } from 'date-fns';

const SUGGESTED_PROMPTS = [
  'What changed significantly today?',
  'Which markets moved the most?',
  'Show me markets with probability below 40%.',
  'Explain the biggest signal today.',
];

interface Source {
  type: string;
  id?: string;
  title?: string;
  market_id?: string;
}

interface CopilotMessage {
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
  sources?: Source[];
}

export default function CopilotPage() {
  const [messages, setMessages] = useState<CopilotMessage[]>([]);
  const [input, setInput] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const send = async () => {
    if (!input.trim() || isLoading) return;
    
    const userMsg = { role: 'user' as const, content: input, timestamp: new Date() };
    setMessages(prev => [...prev, userMsg]);
    setInput('');
    setIsLoading(true);

    try {
      const response = await fetch('/api/v1/copilot/query', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message: input }),
      });

      if (!response.ok) throw new Error('Failed to query copilot');

      const data = await response.json();
      
      const assistantMsg: CopilotMessage = {
        role: 'assistant',
        content: data.answer,
        timestamp: new Date(data.generated_at),
        sources: data.sources || [],
      };
      setMessages(prev => [...prev, assistantMsg]);
    } catch (error) {
      setMessages(prev => [...prev, {
        role: 'assistant',
        content: 'Error: Could not connect to Qevryn Copilot. Please try again.',
        timestamp: new Date(),
      }]);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="space-y-6">
      <Card>
<CardHeader>
  <CardTitle>AI Copilot</CardTitle>
  <CardDescription>Ask Qevryn about prediction markets.</CardDescription>
</CardHeader>
        <CardContent>
          <div className="flex flex-col h-[400px] overflow-auto border rounded-lg p-4 bg-slate-950 space-y-4">
            {messages.length === 0 ? (
              <div className="text-center text-slate-400 mt-10">
                <p className="mb-4">Ask Prophet about prediction markets.</p>
                <div className="flex flex-wrap justify-center gap-2">
                  {SUGGESTED_PROMPTS.map((prompt, i) => (
                    <button
                      key={i}
                      onClick={() => { setInput(prompt); }}
                      className="text-xs bg-slate-800 hover:bg-slate-700 px-3 py-1.5 rounded-full transition-colors"
                    >
                      {prompt}
                    </button>
                  ))}
                </div>
              </div>
            ) : (
              messages.map((msg, i) => (
                <div key={i} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                  <div className={`max-w-[80%] rounded-lg p-4 ${msg.role === 'user' ? 'bg-emerald-600 text-white' : 'bg-slate-800'}`}>
                    <p className="whitespace-pre-wrap">{msg.content}</p>
                    {msg.sources && msg.sources.length > 0 && (
                      <div className="mt-3 pt-3 border-t border-slate-700">
                        <p className="text-xs font-semibold text-slate-400 mb-2">Sources:</p>
                        <div className="flex flex-wrap gap-2">
                          {msg.sources.map((src, idx) => (
                            <Link 
                              key={idx} 
                              href={src.type === 'market' && src.id ? `/markets/${src.id}` : '#'}
                              className="text-xs"
                            >
                              <Badge variant="secondary" className="cursor-pointer hover:bg-slate-700">
                                {src.type}: {src.title || src.id || 'Unknown'}
                              </Badge>
                            </Link>
                          ))}
                        </div>
                      </div>
                    )}
                    <p className="mt-2 text-xs text-slate-500 text-right">
                      {formatDistanceToNow(msg.timestamp)} ago
                    </p>
                  </div>
                </div>
              ))
            )}
            {isLoading && (
              <div className="flex justify-start">
                <div className="bg-slate-800 rounded-lg p-4">
                  <span className="animate-pulse">Thinking...</span>
                </div>
              </div>
            )}
          </div>
          
          <div className="mt-4 flex gap-2">
            <Input
              placeholder="Ask Prophet..."
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && send()}
              disabled={isLoading}
              className="flex-1"
            />
            <Button onClick={send} disabled={isLoading || !input.trim()}>
              Send
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
