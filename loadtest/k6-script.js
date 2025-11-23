import http from "k6/http";
import { check, sleep } from "k6";
import { uuidv4 } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";

export const options = {
  stages: [
    // Нормальная рабочая нагрузка (около 5 rps)
    { duration: "5s", target: 5 },

    // Небольшой стресс-пик
    { duration: "5s", target: 10 },

    { duration: "5s", target: 0 },
  ],

  thresholds: {
    http_req_duration: [
      "p(95)<300",   // Требование SLA 300ms
    ],
    http_req_failed: [
      "rate<0.01",   // <1% ошибок
    ],
  },
};

const BASE_URL = "http://localhost:8080";

export default function () {
  const teamName = "team-" + uuidv4().substring(0, 6);
  const user1 = "u-" + uuidv4().substring(0, 6);
  const user2 = "u-" + uuidv4().substring(0, 6);
  const user3 = "u-" + uuidv4().substring(0, 6);
  const user4 = "u-" + uuidv4().substring(0, 6);
  const prId = "pr-" + uuidv4().substring(0, 6);

  // 1. Создать команду
  let res = http.post(`${BASE_URL}/team/add`, JSON.stringify({
    team_name: teamName,
    members: [
      { user_id: user1, username: "User1", is_active: true },
      { user_id: user2, username: "User2", is_active: true },
      { user_id: user3, username: "User3", is_active: true },
      { user_id: user4, username: "User4", is_active: true },
    ]
  }), { headers: { "Content-Type": "application/json" }});

  check(res, { "team created": (r) => r.status === 201 || r.status === 400 });

  // 2. Создать PR
  res = http.post(`${BASE_URL}/pullRequest/create`, JSON.stringify({
    pull_request_id: prId,
    pull_request_name: "Test PR",
    author_id: user1,
  }), { headers: { "Content-Type": "application/json" }});

  check(res, { "pr created or exists": (r) => r.status === 201 || r.status === 409 });

  // Получаем реального ревьювера для использования в пункте 4
    let assignedReviewer = null;
  if (res.status === 201) {
    const prData = JSON.parse(res.body);
    if (prData.assigned_reviewers && prData.assigned_reviewers.length > 0) {
      assignedReviewer = prData.assigned_reviewers[0];
    }
  }

  // 3. Получить список PR пользователя-ревьювера
  res = http.get(`${BASE_URL}/users/getReview?user_id=${user2}`);
  check(res, { "get review ok": (r) => r.status === 200 || r.status === 404 });

  // 4. Попробовать переназначить ревьювера
  res = http.post(`${BASE_URL}/pullRequest/reassign`,
    JSON.stringify({ pull_request_id: prId, old_user_id: assignedReviewer }),
    { headers: { "Content-Type": "application/json" }}
  );

  check(res, { "reassign allowed or domain error": (r) => [200, 409, 404].includes(r.status) });

  // 5. Пометить как merge (идемпотентно)
  res = http.post(`${BASE_URL}/pullRequest/merge`, JSON.stringify({
    pull_request_id: prId
  }), { headers: { "Content-Type": "application/json" }});

  check(res, { "merged or not found": (r) => r.status === 200 || r.status === 404 });

  sleep(0.2);
}
