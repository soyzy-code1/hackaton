import { Link } from 'react-router-dom';
import { Button } from '@/components/ui/Button';

export function NotFoundPage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-4 text-center">
      <p className="text-sm font-medium uppercase tracking-wide text-gray-500">Ошибка 404</p>
      <h1 className="text-2xl font-semibold tracking-tight text-gray-900">Страница не найдена</h1>
      <p className="max-w-sm text-sm text-gray-500">
        Запрошенная страница не существует или была перемещена.
      </p>
      <Link to="/">
        <Button>На главную</Button>
      </Link>
    </div>
  );
}
