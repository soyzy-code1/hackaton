import { useState } from 'react';
import { Bot, Send } from 'lucide-react';
import { api } from '@/api/client';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';

interface Message {
  role: 'user' | 'assistant';
  content: string;
}

export function AssistantPage() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const sendMessage = async () => {
    const message = input.trim();
    if (!message || isSending) return;
    setInput('');
    setError(null);
    setMessages((current) => [...current, { role: 'user', content: message }]);
    setIsSending(true);
    try {
      const response = await api.assistant.chat(message);
      setMessages((current) => [...current, { role: 'assistant', content: response.message }]);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Не удалось получить ответ ИИ');
    } finally {
      setIsSending(false);
    }
  };

  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-5">
      <div className="flex items-center gap-3">
        <Bot className="h-6 w-6 text-blue-600" aria-hidden="true" />
        <div>
          <h1 className="text-xl font-semibold text-gray-900">Помощь ИИ</h1>
          <p className="text-sm text-gray-500">Чат с GigaChat</p>
        </div>
      </div>

      <section className="card flex min-h-[28rem] flex-col p-4">
        <div className="flex-1 space-y-3 overflow-y-auto" aria-live="polite">
          {messages.length === 0 ? (
            <div className="rounded-lg bg-blue-50 p-4 text-sm text-blue-800">
              Задайте вопрос о планировании расходов или работе с Invoice Keeper.
            </div>
          ) : null}
          {messages.map((item, index) => (
            <div key={`${item.role}-${index}`} className={item.role === 'user' ? 'ml-8 rounded-lg bg-blue-600 p-3 text-sm text-white' : 'mr-8 rounded-lg bg-gray-100 p-3 text-sm text-gray-800'}>
              {item.content}
            </div>
          ))}
          {isSending ? <div className="mr-8 rounded-lg bg-gray-100 p-3 text-sm text-gray-500">GigaChat печатает…</div> : null}
        </div>

        {error ? <p className="mt-3 text-sm text-red-600" role="alert">{error}</p> : null}
        <div className="mt-4 flex items-end gap-2">
          <Input
            label="Сообщение"
            value={input}
            onChange={(event) => setInput(event.target.value)}
            onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); void sendMessage(); } }}
            placeholder="Например: как контролировать расходы?"
            maxLength={4000}
            disabled={isSending}
          />
          <Button onClick={() => void sendMessage()} isLoading={isSending} disabled={!input.trim()} leftIcon={<Send className="h-4 w-4" aria-hidden="true" />}>
            Отправить
          </Button>
        </div>
      </section>
    </div>
  );
}
