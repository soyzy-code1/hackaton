import { useCallback, useEffect, useState } from 'react';
import {
  AlertTriangle,
  ExternalLink,
  Loader2,
  PiggyBank,
  RefreshCw,
  Sparkles,
  TrendingDown,
} from 'lucide-react';
import { api } from '@/api/client';
import type { SubscriptionAlternative, SubscriptionAnalysis } from '@/api/types';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';

const SIMILARITY_LABELS: Record<string, string> = {
  high: 'Полное совпадение',
  medium: 'Совпадает основная функция',
};

function symbol(currency: string): string {
  switch (currency.toUpperCase()) {
    case 'USD':
      return '$';
    case 'EUR':
      return '€';
    case 'GBP':
      return '£';
    default:
      return '₽';
  }
}

function money(amount: number, currency: string): string {
  return `${amount.toFixed(amount % 1 === 0 ? 0 : 2)} ${symbol(currency)}`;
}

function BestAlternativeCard({ item }: { item: SubscriptionAlternative }) {
  return (
    <section className="card overflow-hidden" aria-label="Лучшая альтернатива">
      <div className="border-b border-emerald-200 bg-emerald-50 px-5 py-4">
        <div className="flex items-center gap-2 text-emerald-700">
          <PiggyBank className="h-5 w-5" aria-hidden="true" />
          <span className="text-sm font-semibold uppercase tracking-wide">Лучшая альтернатива</span>
        </div>
        <p className="mt-2 text-lg font-semibold text-gray-900">
          {item.service_name} дешевле, чем {item.payment_title}
        </p>
        <p className="mt-1 text-3xl font-bold text-emerald-600">
          −{money(item.monthly_saving, item.alternative_currency)}
          <span className="ml-2 text-base font-medium text-gray-600">в месяц</span>
        </p>
        <p className="mt-1 text-sm text-gray-600">
          В год: <span className="font-semibold text-gray-900">−{money(item.annual_saving, item.alternative_currency)}</span>
          {item.saving_percent > 0 ? ` (экономия ${item.saving_percent.toFixed(0)}%)` : ''}
        </p>
      </div>
      <div className="grid gap-4 px-5 py-4 sm:grid-cols-2">
        <div className="rounded-lg border border-gray-200 p-3">
          <p className="text-xs text-gray-500">Текущая подписка</p>
          <p className="mt-1 text-sm font-medium text-gray-900">{item.payment_title}</p>
          <p className="text-sm text-gray-600">{money(item.current_price, item.current_currency)} / месяц</p>
        </div>
        <div className="rounded-lg border border-emerald-200 bg-emerald-50/60 p-3">
          <p className="text-xs text-emerald-700">Предлагаемая замена</p>
          <p className="mt-1 text-sm font-medium text-gray-900">{item.service_name}</p>
          <p className="text-sm text-gray-700">{money(item.alternative_price, item.alternative_currency)} / месяц</p>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-2 px-5 pb-4">
        <Badge variant="info">{SIMILARITY_LABELS[item.similarity] ?? item.similarity}</Badge>
        {item.source_url ? (
          <a
            href={item.source_url}
            target="_blank"
            rel="noreferrer noopener"
            className="inline-flex items-center gap-1 text-sm text-blue-700 hover:underline"
          >
            Источник
            <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
          </a>
        ) : null}
      </div>
      {item.reason ? <p className="px-5 pb-4 text-sm text-gray-700">{item.reason}</p> : null}
      {item.evidence ? (
        <p className="mx-5 mb-5 rounded-md bg-gray-50 p-3 text-xs text-gray-600">{item.evidence}</p>
      ) : null}
    </section>
  );
}

function AlternativeRow({ item }: { item: SubscriptionAlternative }) {
  return (
    <li className="border-t border-gray-100 px-5 py-4 first:border-t-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-gray-900">
            {item.service_name} <span className="font-normal text-gray-500">вместо {item.payment_title}</span>
          </p>
          <p className="mt-0.5 text-sm text-gray-600">
            {money(item.alternative_price, item.alternative_currency)} / месяц вместо{' '}
            {money(item.current_price, item.current_currency)}
          </p>
          {item.reason ? <p className="mt-1 text-xs text-gray-500">{item.reason}</p> : null}
        </div>
        <div className="text-right">
          <p className="flex items-center justify-end gap-1 text-sm font-semibold text-emerald-600">
            <TrendingDown className="h-4 w-4" aria-hidden="true" />
            −{money(item.monthly_saving, item.alternative_currency)}
          </p>
          <p className="text-xs text-gray-500">в год −{money(item.annual_saving, item.alternative_currency)}</p>
        </div>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-2">
        <Badge variant="neutral">{SIMILARITY_LABELS[item.similarity] ?? item.similarity}</Badge>
        {item.source_url ? (
          <a
            href={item.source_url}
            target="_blank"
            rel="noreferrer noopener"
            className="inline-flex items-center gap-1 text-xs text-blue-700 hover:underline"
          >
            Источник
            <ExternalLink className="h-3 w-3" aria-hidden="true" />
          </a>
        ) : null}
      </div>
    </li>
  );
}

export function SubscriptionAnalysisPage() {
  const [analysis, setAnalysis] = useState<SubscriptionAnalysis | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const runAnalysis = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const response = await api.assistant.analyzeSubscriptions();
      setAnalysis(response);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Не удалось выполнить анализ подписок');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void runAnalysis();
  }, [runAnalysis]);

  const best = analysis?.best_alternative ?? null;
  const rest = analysis ? analysis.alternatives.filter((item) => item !== best) : [];

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Sparkles className="h-6 w-6 text-blue-600" aria-hidden="true" />
          <div>
            <h1 className="text-xl font-semibold text-gray-900">Анализ подписок</h1>
            <p className="text-sm text-gray-500">
              GigaChat ищет в интернете цены и предлагает более выгодные аналоги
            </p>
          </div>
        </div>
        <Button
          onClick={() => void runAnalysis()}
          isLoading={isLoading}
          leftIcon={<RefreshCw className="h-4 w-4" aria-hidden="true" />}
        >
          Обновить анализ
        </Button>
      </div>

      {error ? (
        <p className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700" role="alert">
          {error}
        </p>
      ) : null}

      {isLoading && !analysis ? (
        <div className="card flex items-center gap-3 p-6 text-sm text-gray-600">
          <Loader2 className="h-5 w-5 animate-spin text-blue-600" aria-hidden="true" />
          Ищу в интернете актуальные цены подписок и сравниваю их с вашими…
        </div>
      ) : null}

      {analysis ? (
        <>
          {best ? (
            <BestAlternativeCard item={best} />
          ) : (
            <section className="card p-5" aria-label="Результат анализа">
              <p className="text-sm text-gray-700">{analysis.summary}</p>
            </section>
          )}

          {analysis.summary && best ? (
            <p className="text-sm text-gray-600">{analysis.summary}</p>
          ) : null}

          {rest.length > 0 ? (
            <section className="card overflow-hidden" aria-label="Другие альтернативы">
              <h2 className="px-5 py-4 text-sm font-semibold text-gray-900">
                Другие выгодные варианты ({rest.length})
              </h2>
              <ul>
                {rest.map((item) => (
                  <AlternativeRow key={`${item.payment_id}-${item.service_name}`} item={item} />
                ))}
              </ul>
            </section>
          ) : null}

          {analysis.warnings.length > 0 ? (
            <section className="card p-4" aria-label="Предупреждения">
              <ul className="space-y-2">
                {analysis.warnings.map((warning) => (
                  <li key={warning} className="flex items-start gap-2 text-sm text-amber-700">
                    <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                    {warning}
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          <section className="card overflow-hidden" aria-label="Проверенные подписки">
            <h2 className="px-5 py-4 text-sm font-semibold text-gray-900">
              Проверено подписок: {analysis.subscriptions.length} · поисковых запросов: {analysis.search_queries}
            </h2>
            <div className="overflow-x-auto">
              <table className="w-full border-collapse text-sm">
                <thead>
                  <tr className="border-b border-gray-200 bg-gray-50">
                    <th className="px-5 py-2 text-left font-medium text-gray-600">Подписка</th>
                    <th className="px-5 py-2 text-left font-medium text-gray-600">Текущая цена</th>
                    <th className="px-5 py-2 text-left font-medium text-gray-600">Найдено цен</th>
                    <th className="px-5 py-2 text-left font-medium text-gray-600">Макс. экономия</th>
                  </tr>
                </thead>
                <tbody>
                  {analysis.subscriptions.map((item) => (
                    <tr key={item.id} className="border-b border-gray-100 last:border-0">
                      <td className="px-5 py-2 text-gray-900">{item.title}</td>
                      <td className="px-5 py-2 text-gray-700">
                        {money(item.amount, item.currency)} / мес
                      </td>
                      <td className="px-5 py-2 text-gray-700">{item.price_hits}</td>
                      <td className="px-5 py-2 text-gray-700">
                        {item.best_saving != null ? `−${money(item.best_saving, 'RUB')}` : '—'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        </>
      ) : null}
    </div>
  );
}
