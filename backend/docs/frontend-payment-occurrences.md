# Контракт payment-occurrences (фронт / mock)

База: `http://localhost:8000`  
Auth: `Authorization: Bearer <token>` после `POST /api/v1/auth/login`.

## Рекомендуемый режим для UI

Query-параметр **`view=minimal`** — ответ только с полями контракта (без `is_overdue`, `days_left`, `recurring_payment_id` и т.д.).

---

## GET `/api/v1/payment-occurrences?view=minimal`

**Фильтры (опционально):** `currency`, `status` (`paid` | `unpaid` | `overdue`), `recurring_payment_id`, `from_date`, `to_date`.

**Ответ `200`:** `PaymentOccurrenceMinimal[]`

| Поле | Тип | Описание |
|------|-----|----------|
| `id` | integer | ID периода |
| `title` | string | Название регулярного платежа |
| `amount` | string | Сумма, 2 знака (`"15000.00"`) |
| `currency` | string | `RUB`, `USD`, `EUR` |
| `due_date` | string | Срок оплаты `YYYY-MM-DD` |
| `is_paid` | boolean | Оплачен ли период |
| `actual_payment_date` | string \| null | Фактическая дата оплаты |

```http
GET /api/v1/payment-occurrences?view=minimal
Authorization: Bearer …
```

```json
[
  {
    "id": 1,
    "title": "Аренда",
    "amount": "15000.00",
    "currency": "RUB",
    "due_date": "2026-11-01",
    "is_paid": false,
    "actual_payment_date": null
  }
]
```

Без `view=minimal` возвращается расширенный объект (`paid_at`, `is_overdue`, `days_left`, …).

---

## PATCH `/api/v1/payment-occurrences/{id}?view=minimal`

**Тело — отметить оплату:**

```json
{
  "is_paid": true,
  "actual_payment_date": "2026-10-28"
}
```

| Поле | Обязательно | Примечание |
|------|-------------|------------|
| `is_paid` | да | `true` / `false` |
| `actual_payment_date` | нет* | *При `is_paid: true` — если не передано, подставляется сегодня (`TIMEZONE`) |

Алиас запроса: `paid_at` вместо `actual_payment_date`.

**Тело — снять оплату:**

```json
{ "is_paid": false }
```

**Ответ `200`:** `PaymentOccurrenceMinimal` (если `view=minimal`) или полный `PaymentOccurrenceOut`.

**Ошибки:** `401`, `404`, `422` (дата в будущем).

---

## Mock без backend

Статический пример: [`mock/payment-occurrences.sample.json`](../mock/payment-occurrences.sample.json).

Поведение mock:

- **GET** — отдать массив из файла.
- **PATCH** — найти элемент по `id`, выставить `is_paid` и `actual_payment_date`, вернуть объект.

---

## OpenAPI

Схемы: `PaymentOccurrenceMinimal`, `PaymentOccurrencePatch` в [`openapi.yaml`](../openapi.yaml).  
Swagger: `/api/v1/docs`.
