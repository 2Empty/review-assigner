# Результаты нагрузочного тестирования

Тестирование проводилось с помощью скрипта `loadtest/k6-script.js` с параметрами:

- Сценарий: постепенный рост нагрузки до 10 виртуальных пользователей (VU)
- Продолжительность теста: 15 секунд с тремя этапами (нагрузка, удержание, спуск)
- Цели (thresholds):
  - Время ответа (p95) должно быть меньше 300 мс
  - Доля неудачных запросов (ошибок) должна быть менее 1%

---

## Основные метрики

- **Общее количество проверок**: 1795 (100% успешных)
- **Среднее время ответа**: 1.97 мс
- **95-й процентиль времени ответа**: 3.24 мс (значительно ниже порога 300 мс)
- **Процент ошибок**: 0.00% (0 ошибок из 1795 запросов)
- **Запросов в секунду**: ~120 запросов/сек

---

## Вывод

Система успешно выдерживает нагрузку до 10 одновременных пользователей и отвечает быстро и стабильно. Ошибки встречаются крайне редко и не превышают установленные пороги качества.

---

## Запуск теста

make loadtest

---

## Мой пример выполнения

     execution: local
        script: loadtest/k6-script.js
        output: -

     scenarios: (100.00%) 1 scenario, 10 max VUs, 45s max duration (incl. graceful stop):
              * default: Up to 10 looping VUs for 15s over 3 stages (gracefulRampDown: 30s, gracefulStop: 30s)



    THRESHOLDS 

    http_req_duration
    ✓ 'p(95)<300' p(95)=3.24ms

    http_req_failed
    ✓ 'rate<0.01' rate=0.00%


    TOTAL RESULTS 

    checks_total.......: 1795    119.49462/s
    checks_succeeded...: 100.00% 1795 out of 1795
    checks_failed......: 0.00%   0 out of 1795

    ✓ team created
    ✓ pr created or exists
    ✓ get review ok
    ✓ reassign allowed or domain error
    ✓ merged or not found

    HTTP
    http_req_duration..............: avg=1.97ms   min=463.89µs med=2.17ms   max=32.75ms  p(90)=3.05ms   p(95)=3.24ms 
      { expected_response:true }...: avg=1.97ms   min=463.89µs med=2.17ms   max=32.75ms  p(90)=3.05ms   p(95)=3.24ms 
    http_req_failed................: 0.00%  0 out of 1795
    http_reqs......................: 1795   119.49462/s

    EXECUTION
    iteration_duration.............: avg=211.01ms min=209.41ms med=210.83ms max=241.71ms p(90)=211.78ms p(95)=212.1ms
    iterations.....................: 359    23.898924/s
    vus............................: 1      min=1         max=9 
    vus_max........................: 10     min=10        max=10

    NETWORK
    data_received..................: 578 kB 39 kB/s
    data_sent......................: 396 kB 26 kB/s




    running (15.0s), 00/10 VUs, 359 complete and 0 interrupted iterations
    default ✓ [======================================] 00/10 VUs  15s