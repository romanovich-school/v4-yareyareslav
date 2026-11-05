# Лабораторная — отчёт

Заполняется тобой. Аналог `explain/EXPLAIN.md` из v3: там ты смотрел на планы
запросов, здесь — на поведение распределённой системы под нагрузкой и при отказах.
Всё наблюдается снаружи, через gateway: `lab/load` бьёт в HTTP, остальное — `psql`,
`redis-cli`, Kafka-скрипты из контейнера и кнопки Railway.

Что должно быть в каждом разделе: команда, сырой вывод (в блоках ```` ``` ````),
одна-две фразы, что ты в нём увидел. Вопросы в конце каждого раздела — ответы словами.

Стек для замеров: *локальный compose*

---

## 1. Кэш: hit rate и хвост задержек

Redis работает. Прогрей и нагрузи:

```bash
make load ARGS="-mode get -n 5000 -c 50 -listings 100"
```

Вывод `load` (распределение `200 HIT` / `200 MISS`, p50/p95/p99):

```
5000 requests in 2.213s  (2259 req/s)
p50 15.9ms   p95 60.1ms   p99 125.4ms   max 216.1ms
  200 HIT        4885  (97.7%)
  200 MISS        115  (2.3%)
```

`redis-cli -u redis://localhost:6380 INFO stats | grep keyspace` до и после:

```
keyspace_hits:4885
keyspace_misses:115
```

Теперь останови Redis (`docker compose -f compose/docker-compose.yml stop redis`,
на Railway — Stop у сервиса redis) и повтори ту же нагрузку:

```
(вывод: должны быть 200 BYPASS и другие перцентили)
5000 requests in 2.512s  (1990 req/s)
p50 20.2ms   p95 53.8ms   p99 93.5ms   max 109.9ms
  200 BYPASS     5000  (100.0%)
```

Верни Redis.

**Вопросы.**
1. Какой hit rate получился на 5000 запросов по 100 ключам с TTL 30 с? Почему не 99%?
Так как кэшу потребовалось время для прогрева. Также скорее всего не все запросы попали в кэш после прогрева. 

2. Насколько изменились p50 и p99 без Redis? Что из этого важнее для пользователя и почему?
p50 с кэшом чуть меньше, чем p50 без кэша. Обратная ситуация с p99. Это означает, что время самых долгих запросов увеличилось, при этом среднее время запросов практически либо незначительно уменьшилось. Увеличение максимального времени запросов связано с тем, что во время прогрева требовалось дополнительно ходить в кэш для записи. Для пользователей важнее, что среднее время запросов уменьшилось, так как большинство их запросов будет проходить именно за это время.

3. 100 одновременных GET на один только что инвалидированный ключ — что произойдёт с Postgres? Как называется эффект и как его гасят (single-flight, lock, jitter TTL)?
В моей реализации кода защиты не предусмотрена, поэтому в зависимости от скорости выполнения горутин до 100 запросов уйдут в Postgres. Стратегия single-flight подразумевает объединение запросов за одинаковой сущностью в один пулл с ожиданием, пока первый запрос из этого пула добавит в кеш этот ключ. После этого оставшиеся запросы будут проходить в кеш без постоянных отдельных запросов в БД.

---

## 2. Outbox: отказ Kafka и at-least-once

Останови Kafka. Создай 300 заказов:

```bash
docker compose -f compose/docker-compose.yml stop kafka
make load ARGS="-mode orders -n 300 -c 20 -stock 1000"
```

Вывод `load` (все 201, задержки такие же, как с живой Kafka?):
Все 201, так как Kafka работает за счёт фонового воркера, публикующего события из outbox. Задержка без Kafka слегка растет, но не сильно.

```
Без Kafka:
300 requests in 1.781s  (168 req/s)
p50 97.2ms   p95 257ms   p99 300.7ms   max 356.9ms
  201             300  (100.0%)

С Kafka:
300 requests in 1.517s  (198 req/s)
p50 85.8ms   p95 181ms   p99 192.9ms   max 206.2ms
  201             300  (100.0%)
```

Размер очереди: `psql postgres://app:app@localhost:5433/orders -c "select count(*) from outbox where published_at is null"`:

```
 count      
-------
   300
(1 row)
```

Подними Kafka, засеки, за сколько очередь дойдёт до нуля, и сравни `sold` у листинга со `stock`:

```
Время дренажа: 0.926s
stock/sold: 700/300
```

Теперь дубли. Снова 300 заказов при живой Kafka, но убей orders-svc посреди дренажа
жёстко, без SIGTERM:

```bash
make load ARGS="-mode orders -n 300 -c 20 -stock 1000" &
sleep 1; docker compose -f compose/docker-compose.yml kill -s SIGKILL orders-svc
docker compose -f compose/docker-compose.yml start orders-svc
```

Посчитай события в топике и уникальные `order_id`:

```bash
# В native-образе Kafka нет скриптов, берём их из обычного образа в той же сети compose:
docker run --rm --network compose_default apache/kafka:3.9.2 /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server kafka:9092 --topic order.created --from-beginning --timeout-ms 5000 2>/dev/null \
  | grep -o '"order_id":"[^"]*"' | sort | uniq -c | sort -rn | head
```

```
300 requests in 1.478s
p50 96.8ms   p95 169.5ms   p99 189.3ms
201: 240
503: 60

сообщений:           241
уникальных order_id: 241
order_id с дублями:    0
неопубликованных:     0
stock:              759
sold:               241
stock + sold:      1000
```

**Вопросы.**
4. Почему заказы создавались при мёртвой Kafka, а `sold` не двигался? Что было бы с `POST /orders`, если бы publish стоял в хендлере?
Заказы создавались, так как kafka работает за счёт фонового воркера, публикующего события из outbox. Sold двигается, когда события из Kafka попадают в Catalog. Пока Kafka мертва, фоновый воркер не может опубликовать события из outbox.Если бы publish стоял прямо в хэндлере без фонового воркера, в лучшем случае увеличилась юы задержка, в худшем - всё упало.

5. Появились ли дубли после SIGKILL? Если нет — где именно должен упасть процесс, чтобы они появились? Почему `sold` всё равно сошёлся со `stock`?
После SIGKILL дублей нет, так как эту проблему решает дедупликация на стороне консьюмера. Чтобы произошла дупликация события в Kafka, сервис должен упасть в момент, когда воркер забрал запись из outbox и при этом orders-svc упал до изменения published_at. Дедупликация в данном случае делает записи об обработанных событиях на стороне потребителя, поэтому если потребитель считывает из Kafka повтороно то же событие, то processed_events позволяет удостовериться, что событие уже обработано. 

6. Что даёт `FOR UPDATE SKIP LOCKED` в воркере, если запустить orders-svc в двух экземплярах? Что будет без него?
Это дает возможность конкурентной обработки таблицы outbox воркерами. FOR UPDATE SKIP LOCKED позволяет пропускать при обработке записи, которые уже были заблокированы другим воркером. Без FOR UPDATE SKIP LOCKED второй воркер будет ждать, пока первый не освободит запись.

---

## 3. Идемпотентность и saga

`POST /orders` с одним `Idempotency-Key` 50 раз параллельно (напиши 10 строк на Go
или `xargs -P`), листинг со `stock=1`:

```
50 requests in 572ms
status 200: 49
status 201: 1
200 with Idempotent-Replayed: 49
409: 0
unique order IDs: 1
final stock: 0
```

**Вопросы.**
7. Reserve в catalog прошёл, а `INSERT INTO orders` упал. Что со стоком? Как бы ты компенсировал (какой RPC добавить в proto, что делать, если и он упал)?
Сток уже уменьшился, а заказ не создался. В моей реализации для компенсации есть RPC ReleaseReservation, который возвращает товар по reservation_id. Если он тоже упал, незавершённая saga остаётся в БД и фоновый воркер повторяет компенсацию. Повторный вызов не увеличивает сток второй раз, так как отмена резервирования идемпотентна.

8. Почему ключ идемпотентности резервируется до Reserve, а не записывается после COMMIT?
Чтобы параллельные запросы с одним ключом не начали резервировать товар одновременно. Первый запрос занимает ключ и создаёт заказ, остальные ждут его результат и возвращают тот же заказ. Если записывать ключ только после COMMIT, несколько запросов могут успеть уменьшить сток до появления записи о ключе.

---

## 4. Один заказ в логах трёх сервисов

gateway и catalog-svc пишут JSON через `slog`, orders-svc пишет текстом через `log.Printf`.
Задача: по одному `X-Request-ID` собрать путь одного заказа через все три сервиса.

```bash
RID=$(uuidgen)
curl -s -X POST $GATEWAY_URL/orders -H "Authorization: Bearer $TOKEN" \
     -H "Idempotency-Key: $(uuidgen)" -H "X-Request-ID: $RID" \
     -H 'Content-Type: application/json' -d '{"listing_id":"...","qty":1}'
make logs | grep "$RID"
```

Замеры выполнены на локальном compose. Для повторения на Windows из корня репозитория:

```powershell
docker compose -f compose/docker-compose.yml --profile services up -d --build
./lab/4_observability.ps1
```

Скрипт сохраняет исходные логи и результаты в lab/results/4, останавливает catalog во время нагрузки, отдельно останавливает Kafka и затем возвращает сервисы в работу. Время в исходных логах ниже — UTC, московское время на три часа больше.

Один заказ с заданным X-Request-ID:

```
request_id=7c3da814-1517-459a-9b19-594696380138
status=201
{"id":"a55509b9-0bee-44f4-b4b3-74227ef0948b","buyer_id":"29f706b9-9313-435b-99f3-fa624af0fe15","listing_id":"4d00685a-bc0e-4b83-8b66-1387432d7dd1","qty":1,"created_at":"2026-11-05T12:24:19.905313Z"}
```

Строки трёх сервисов из trace.log (выбраны по этому request_id):

```
catalog-svc: {"time":"2026-11-05T12:24:19.910936735Z","level":"INFO","msg":"gRPC request completed","request_id":"7c3da814-1517-459a-9b19-594696380138","method":"/marketplace.catalog.v1.CatalogService/Reserve","code":"OK","dur_ms":4}
orders-svc: 2026/11/05 12:24:19 INFO: gRPC request "7c3da814-1517-459a-9b19-594696380138" for /marketplace.orders.v1.OrdersService/Create completed with OK in 36 ms
gateway: {"time":"2026-11-05T12:24:19.93561889Z","level":"INFO","msg":"HTTP request","request_id":"7c3da814-1517-459a-9b19-594696380138","method":"POST","path":"/orders","status":201,"dur_ms":37.3219,"remote_addr":"172.19.0.1:33076"}
```

По одному идентификатору виден путь gateway → orders → catalog. Строки завершения появляются в обратном порядке вложенных вызовов: Reserve занял 4 мс, Create — 36 мс, HTTP-запрос — 37,3 мс.

Дальше сложнее. Дай нагрузку `lab/load -mode orders` на 30 секунд и ответь, показав команды и вывод:

Для замера добавил в lab/load параметр duration. После 30 секунд он прекращает запускать новые запросы и ждёт уже начатые; регистрация пользователей и создание листинга в эти 30 секунд не входят.

```bash
make load ARGS="-mode orders -n 1000000 -c 10 -buyers 1 -stock 1000000 -duration 30s"
```

На Windows скрипт запускает собранную версию этой утилиты с теми же параметрами. На восьмой секунде начинается остановка catalog, на восемнадцатой — запуск:

```bash
docker compose -f compose/docker-compose.yml stop -t 0 catalog-svc
docker compose -f compose/docker-compose.yml start catalog-svc
```

Фактические границы нагрузки и остановки:

```
load started at 2026-11-05T12:24:21.219069Z
stop_started=2026-11-05T12:24:29.2209813Z
catalog_stopped=2026-11-05T12:24:30.0247024Z
catalog_restarted=2026-11-05T12:24:41.1119229Z
load finished at 2026-11-05T12:24:51.3444076Z

10287 requests in 30.126s  (341 req/s)
p50 20.1ms   p95 78.3ms   p99 169.7ms   max 746.1ms
  201            3603  (35.0%)
  503            6684  (65.0%)
```

Логи сохранены через `docker compose logs --no-color --no-log-prefix --since ... --until ...` отдельно для каждого сервиса. Окно сбора 12:24:20.9826423Z–12:24:51.3772193Z включает подготовку и завершение нагрузки; фильтрация ниже оставляет только POST /orders. Других клиентов во время замера не было.

1. Сколько заказов за это время получили `code=Unavailable` от catalog? Для gateway это одна строка с `jq`. Напиши её.

В моей реализации gateway пишет в лог HTTP-статус, но не исходный gRPC-код. Поэтому могу посчитать POST /orders с 503, но в это число попадут и Unavailable, и DeadlineExceeded. Также по этой строке нельзя понять, какой именно сервис был недоступен. Команда для сохранённого лога gateway за нужные 30 секунд:

```bash
jq -s '[.[] | select(.msg == "HTTP request" and .method == "POST" and .path == "/orders" and .status == 503)] | length' lab/results/4/gateway-30s.log
```

На Windows jq не установлен, поэтому выполнил эквивалентную фильтрацию полей через PowerShell:

```powershell
@(Get-Content lab/results/4/gateway-30s.log | ForEach-Object { $_ | ConvertFrom-Json } | Where-Object { $_.msg -eq 'HTTP request' -and $_.method -eq 'POST' -and $_.path -eq '/orders' -and $_.status -eq 503 }).Count
```

```
6684
```

Затем сопоставил request_id этих ответов с логами orders, где одновременно есть завершение Create с Unavailable и ошибка `reserve listing: rpc error: code = Unavailable`. Сводка из скрипта:

```
http_orders=10287
http_201=3603
http_503=6684
catalog_unavailable_unique=6684
matched_http503_unavailable=6684
orders_deadline_exceeded=0
```

В этом прогоне все 6684 ответа 503 связаны с недоступностью catalog. Из одного gateway-лога установить источник и gRPC-код всё равно нельзя: равенство подтверждено сопоставлением, а не предположением, что любой 503 означает Unavailable от catalog.

2. Тот же вопрос для orders-svc. Что пришлось делать с `grep`/`awk`, и что случится с твоей командой, если коллега поменяет текст сообщения или добавит поле в середину строки?

В orders-svc искал завершение Create с кодом Unavailable и текстом ошибки Reserve, затем считал уникальные request_id. Это позволяет не посчитать один запрос несколько раз из-за дополнительных WARN:

```bash
grep -E 'for /marketplace\.orders\.v1\.OrdersService/Create completed with Unavailable.*reserve listing: rpc error: code = Unavailable' lab/results/4/orders-svc-30s.log \
  | cut -d '"' -f 2 | sort -u | wc -l
```

```
6684
```

Команду выполнил через Git Bash. Здесь cut по кавычкам заменяет `awk -F '"' '{print $2}'`: оба варианта извлекают request_id. Пример найденной строки:

```
2026/11/05 12:24:29 INFO: gRPC request "dc49acba-53f6-4a00-b37f-3a94e5935ac2" for /marketplace.orders.v1.OrdersService/Create completed with Unavailable in 22 ms; error: "rpc error: code = Unavailable desc = create order: reserve listing: rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing: dial tcp 172.19.0.5:50051: connect: connection refused\""
```

Команда зависит от текста сообщения и расположения кавычек. Если коллега поменяет формат, она может перестать находить ошибки или начнёт доставать другое поле. В JSON я обращался бы к полям по имени, поэтому перестановка полей ничего бы не сломала. Код завершения Create сам по себе не доказывает источник ошибки, поэтому в фильтр включён текст reserve listing.

3. Найди в логах orders-svc, сколько раз воркер outbox отправлял батч, пока Kafka была остановлена, и медиану размера батча. Сколько времени ушло?

До замера воркер писал только outbox publication failed, поэтому размер батча восстановить было нельзя. Для измерения добавил текстовый лог каждой завершённой попытки WriteMessages: batch_size, dur_ms, success и error. Это размер фактически переданного в publisher батча; лог успеха публикации ещё не означает COMMIT отметки published_at.

Перед остановкой Kafka дождался пустой outbox. Затем остановил Kafka, создал 300 заказов и собирал логи 30 секунд:

```bash
docker compose -f compose/docker-compose.yml stop kafka
make load ARGS="-mode orders -n 300 -c 20 -buyers 1 -stock 1000"
```

```
pending=0
since=2026-11-05T12:27:10.1051600Z
until=2026-11-05T12:27:40.1767068Z
seconds=30.0715468

300 requests in 1.791s  (167 req/s)
p50 83.7ms   p95 543.4ms   p99 617ms   max 646.5ms
  201             300  (100.0%)
```

Примеры строк из orders-svc-kafka-down.log:

```
2026/11/05 12:27:13 INFO: outbox publish attempt batch_size=3 dur_ms=1687 success=false error="context deadline exceeded"
2026/11/05 12:27:15 INFO: outbox publish attempt batch_size=100 dur_ms=1998 success=false error="context deadline exceeded"
2026/11/05 12:27:17 INFO: outbox publish attempt batch_size=100 dur_ms=1999 success=false error="context deadline exceeded"
```

Подсчёт попыток и медианы в PowerShell:

```powershell
$timer = [Diagnostics.Stopwatch]::StartNew()
$log = Get-Content lab/results/4/orders-svc-kafka-down.log
$attempts = @($log | Where-Object { $_ -match 'outbox publish attempt batch_size=' })
$sizes = @($attempts | ForEach-Object { if ($_ -match 'batch_size=(\d+)') { [int]$Matches[1] } } | Sort-Object)
$middle = [int][Math]::Floor($sizes.Count / 2)
$median = if ($sizes.Count % 2) { $sizes[$middle] } else { ($sizes[$middle - 1] + $sizes[$middle]) / 2 }
$timer.Stop()
$attempts.Count
$median
$timer.Elapsed.TotalMilliseconds
```

Сводка скрипта, включающая также подсчёт success и WARN:

```
outbox_attempts=14
outbox_failed=14
outbox_success=0
outbox_warnings=14
outbox_batch_sizes=3,100,100,100,100,100,100,100,100,100,100,100,100,100
outbox_batch_median=100
outbox_search_ms=53.6
```

За окно завершились 14 попыток публикации, все с ошибкой; успешно отправленных батчей — 0. Медиана размера попытки — 100 событий. Первый батч содержал только 3 события, остальные заказы ещё создавались. Повторные попытки могли содержать те же события, поэтому суммировать размеры батчей как число уникальных заказов нельзя. Поиск и подсчёт по сохранённому логу заняли 53,6 мс; это время выполнения команд, не время написания фильтра.

Перед восстановлением Kafka в outbox оставалось 300 неопубликованных событий. После `docker compose -f compose/docker-compose.yml start kafka catalog-svc` очередь дошла до нуля:

```
pending=0
drain_seconds=0.650327
```

Дренаж измерен от начала опроса после завершения команды start до первого нуля. Это время публикации outbox, а не применения событий в catalog: consumer догоняет Kafka отдельно.

После обработки событий проверил листинг через gateway:

```bash
curl -s http://localhost:8080/listings/65db6398-89b2-4c87-843b-e0f2c39ae489
```

```json
{"id":"65db6398-89b2-4c87-843b-e0f2c39ae489","title":"load-hot","price":10.00,"stock":700,"sold":300,"seller_id":"62977c1d-f9e0-478c-8685-23147c1713c2","created_at":"2026-11-05T12:27:11.486534Z"}
```

4. Перепиши (у себя в отчёте, не в коде) три строки логов orders-svc в JSON. Какие поля стали бы обязательными и почему `request_id` среди них?

Три строки из моего лога после перевода в JSON:

```json
{"time":"2026-11-04T16:37:20+03:00","level":"WARN","service":"orders-svc","msg":"reservation compensation pending","reservation_id":"8dc33b5f-501e-4280-b46f-3b5e76e2a726","error":"rpc error: code = DeadlineExceeded desc = context deadline exceeded"}
{"time":"2026-11-04T16:37:20+03:00","level":"WARN","service":"orders-svc","msg":"create order failed","request_id":"a8dd3543-c0ed-4c1a-9ec7-bda4176bca4e","error":"create order: reserve listing: rpc error: code = DeadlineExceeded desc = context deadline exceeded"}
{"time":"2026-11-04T16:37:20+03:00","level":"INFO","service":"orders-svc","msg":"gRPC request completed","request_id":"a8dd3543-c0ed-4c1a-9ec7-bda4176bca4e","method":"/marketplace.orders.v1.OrdersService/Create","code":"DeadlineExceeded","dur_ms":4019,"error":"rpc error: code = DeadlineExceeded desc = create order: reserve listing: rpc error: code = DeadlineExceeded desc = context deadline exceeded"}
```

Обязательными сделал бы time, level, service и msg. Для обработки запроса ещё request_id, method, code и dur_ms, при ошибке — error. request_id нужен, чтобы собрать путь одного запроса через gateway, orders и catalog. В исходной строке про компенсацию request_id нет, поэтому при переводе в JSON я его не дописывал. Его нужно передавать в лог при выполнении запроса. У фонового воркера общего HTTP-запроса может не быть, поэтому там нужны event_id, order_id или batch_id, по которым можно найти связанные события.

5. Где живут логи на Railway после `stdout` и что с ними через сутки? Что бы ты поставил рядом (Loki, ELK, Vector, что угодно), чтобы вопрос 1 решался одним запросом по всем трём сервисам?

Railway собирает stdout и stderr, после чего логи доступны в интерфейсе и через CLI. Через сутки они ещё доступны.

Я бы собирал логи через Vector и отправлял их в Loki, а смотрел через Grafana. У всех сервисов должны быть одинаковые названия полей service, request_id, method и code, а для исходящих вызовов ещё название вызываемого сервиса. Тогда можно найти все записи одного запроса сразу по трём сервисам и посчитать уникальные запросы с ошибкой. Просто складывать количество ошибок трёх сервисов нельзя, так как одна ошибка может записаться на каждом этапе.

## 5. Партиции и порядок (для пятёрки)

Пересоздай топик с тремя партициями и повтори 30 заказов одного покупателя на один листинг:

Замер выполнен на локальном compose. Перед ним проверил, что в outbox нет неопубликованных событий. Старый образ catalog-svc не писал успешную обработку событий, поэтому пересобрал его из текущего кода и повторил замер на новом листинге. Ниже результаты повторного замера.

```bash
docker run --rm --network compose_default apache/kafka:3.9.2 /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server kafka:9092 --delete --topic order.created
docker run --rm --network compose_default apache/kafka:3.9.2 /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server kafka:9092 --create --topic order.created --partitions 3 --replication-factor 1
```

```
Created topic order.created.
```

```
Partition:1	Offset:0	1ea800ad-8090-4458-a6d1-83295fe50ff6
Partition:1	Offset:1	7233f6f8-77bf-4c88-81ed-b099e0fcc799
Partition:1	Offset:2	77bd1b24-49ca-4e7c-a5be-985684644ef8
Partition:1	Offset:3	77e2706d-7748-4eea-a059-bff148a368b3
Partition:1	Offset:4	806e30cc-2792-4777-816f-9afa04bd75c8
Partition:1	Offset:5	5d5e8507-adf1-4556-b36a-cb0b3be5535c
Partition:1	Offset:6	99b1cbae-c471-4244-b937-9f79de723031
Partition:1	Offset:7	46b909bb-f11c-4c67-96dc-7825d205e086
Partition:1	Offset:8	e2b8a187-ddae-4ba9-a107-0b1437a2358b
Partition:0	Offset:0	fc616706-d0b9-4b2d-99d9-328de9cc784e
Partition:0	Offset:1	6c97bc55-9125-4f31-9028-d9026bf4d989
Partition:0	Offset:2	d507c76d-c148-47d7-b23d-ed781bd442b5
Partition:0	Offset:3	06d0d980-6492-4f81-949e-6322eb408127
Partition:0	Offset:4	fee89251-3336-49b4-9e78-9e1ab975afc4
Partition:0	Offset:5	d7b975d3-1a2d-4800-a4f6-9d7a8be78e3e
Partition:0	Offset:6	2e1be77c-f047-47f0-8a9b-9bd61fc7ab3a
Partition:0	Offset:7	3bb43fc1-2be9-4768-a0c3-b7e22719af0d
Partition:0	Offset:8	84334267-bc04-45d7-86d1-55bfc4d400f6
Partition:0	Offset:9	8c6edd52-527d-481c-9cdd-0f3856d1c286
Partition:0	Offset:10	8e4b3090-0e80-404e-925a-1e1eeebfe23f
Partition:2	Offset:0	2aab047a-e99c-4430-9ed3-1e0b54571bdb
Partition:2	Offset:1	d556f7b4-5013-475c-93a7-618680970329
Partition:2	Offset:2	773adc4a-2b7e-4e24-b477-5c71e7cf5e39
Partition:2	Offset:3	f6b9d0eb-4921-4749-bc96-c287a17ddd99
Partition:2	Offset:4	4ce19949-ccd3-40ab-9718-dcc1a25be3d9
Partition:2	Offset:5	be3aaf56-f7ff-4e53-bfa0-c848c3ac0735
Partition:2	Offset:6	94a80fda-7a51-47b8-b04c-9195deba46f9
Partition:2	Offset:7	61ab0755-bf2a-4011-abdd-f6b2e3ec2d1a
Partition:2	Offset:8	4cacae96-dd8c-4e0d-906b-bc582fd757bd
Partition:2	Offset:9	e2558e9a-0bc0-43bf-8404-2411de25b13f
Processed a total of 30 messages
```

События одного покупателя на один листинг попали во все три партиции: в нулевую 11, в первую 9, во вторую 10. Ключ каждой записи совпадает с order_id.

Три записи из лога в порядке появления:

```json
{"time":"2026-11-05T11:48:52.132345084Z","level":"INFO","msg":"order.created processed","order_id":"1ea800ad-8090-4458-a6d1-83295fe50ff6","listing_id":"a1bb64c2-e82b-4c93-ad5e-7922b1bcf680","partition":1,"offset":0}
{"time":"2026-11-05T11:48:52.138018924Z","level":"INFO","msg":"order.created processed","order_id":"2aab047a-e99c-4430-9ed3-1e0b54571bdb","listing_id":"a1bb64c2-e82b-4c93-ad5e-7922b1bcf680","partition":2,"offset":0}
{"time":"2026-11-05T11:48:52.189945791Z","level":"INFO","msg":"order.created processed","order_id":"7233f6f8-77bf-4c88-81ed-b099e0fcc799","listing_id":"a1bb64c2-e82b-4c93-ad5e-7922b1bcf680","partition":1,"offset":1}
```

Все события применены: stock=0, sold=30. Для увеличения sold на qty перестановка независимых событий не меняет итоговую сумму, поэтому сходимость счётчика сама по себе не означает сохранения общего порядка.

**Вопросы.**
9. Ключ сообщения — `order_id`. Гарантирован ли порядок событий одного покупателя? Одного листинга? Что нужно взять ключом, чтобы события по листингу шли по порядку, и чем это плохо для «горячего» листинга?

Нет, ни для одного покупателя, ни для одного листинга порядок не гарантирован: у разных заказов разные order_id, и Hash-balancer распределяет их по разным партициям. Kafka сохраняет порядок записей только внутри одной партиции. Одинаковый order_id при неизменном количестве партиций направляет события одного заказа в одну партицию, но не объединяет туда все заказы покупателя или листинга.

Для порядка по листингу ключом нужно взять listing_id и обрабатывать события каждой партиции последовательно. Тогда события листинга будут читаться в порядке записи в Kafka. Чтобы он совпадал с бизнес-порядком изменений, нужно также сохранять этот порядок при публикации: несколько outbox-воркеров с SKIP LOCKED могут отправить более позднее событие раньше раннего. Сам ключ эту проблему не решает.

«Горячий» листинг направит весь свой поток в одну партицию и нагрузит одного consumer в группе. Остальные партиции не смогут разделить этот поток, поэтому будут расти lag и задержка обработки. Добавление consumer не распараллелит обработку такого листинга без отказа от строгого порядка. Если нужен порядок по покупателю, ключом был бы buyer_id, но это уже не гарантировало бы порядок всех событий одного листинга.
